package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bww/go-util/v1/ext"
)

// Manages rate limiting state by evaluating response headers. This is used for
// respectable services that could be bothered to implement something like the
// 'RateLimit Fields for HTTP' draft standard:
//
// https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers
//
// If similar implementations are encountered which happen to use different
// header names or time/duration formats, it would be reasonable to update this
// implementation to accommodate them.
type headers struct {
	impl limiter
	dur  Durationer
}

func NewHeaders(conf Config) *headers {
	var dur Durationer
	if d := conf.Durationer; d != nil {
		dur = d
	} else {
		dur = Seconds
	}
	return &headers{
		impl: limiter{
			limit:         conf.Events,
			remaining:     conf.Events,
			reset:         ext.Coalesce(conf.Start, time.Now()).Add(conf.Window),
			mode:          conf.Mode,
			maxMeter:      conf.MaxDelay,
			backoffPeriod: defaultBackoffPeriod,
		},
		dur: dur,
	}
}

func (l *headers) Next(rel time.Time, opts ...Option) (time.Time, error) {
	conf := Options{}.With(opts)
	if conf.Attrs == nil {
		return time.Time{}, fmt.Errorf("%w: Header attributes are required", ErrMissingAttrs)
	}
	delay, err := l.impl.Delay(rel)
	if err != nil {
		return time.Time{}, fmt.Errorf("Could not compute next window: %w", err)
	}
	if delay > 0 {
		return rel.Add(delay), nil
	} else {
		return rel, nil
	}
}

func (l *headers) Wait(cxt context.Context, rel time.Time, opts ...Option) (time.Time, error) {
	t, err := l.Next(rel, opts...)
	if err != nil {
		return time.Time{}, err
	}
	if !t.After(rel) { // the next window is at or before the reference time: don't wait
		return rel, nil
	}
	select {
	case <-time.After(t.Sub(rel)):
		return t, nil
	case <-cxt.Done():
		return t, ErrCanceled
	}
}

func (l *headers) State(time.Time) State {
	return l.impl.State()
}

func (l *headers) Update(rel time.Time, opts ...Option) error {
	conf := Options{}.With(opts)
	if conf.Attrs == nil {
		return fmt.Errorf("%w: Header attributes are required", ErrMissingAttrs)
	}
	return l.update(rel, conf.Attrs)
}

func (l *headers) update(rel time.Time, attrs Attrs) error {
	var lim, rem int
	var rst time.Time
	var err error

	// retry-after may be present even when other rate limit headers are not, handle it first
	if n, v := findAttr(attrs, "X-Retry-After", "Retry-After"); v != "" {
		x, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("Rate limit header is invalid: %s = %s: %v", n, v, err)
		}
		w := time.Now().Add(l.dur.Duration(x))
		l.impl.BackoffUntil(w)
		return RetryError{
			RetryAfter: w,
		}
	}

	if n, v := findAttr(attrs, "X-RateLimit-Limit", "ratelimit-limit"); v == "" {
		return fmt.Errorf("No quota limit header: %w", ErrMissingHeaders)
	} else {
		lim, err = strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("Rate limit header is invalid: %s = %s: %v", n, v, err)
		}
	}

	if n, v := findAttr(attrs, "X-RateLimit-Remaining", "ratelimit-remaining"); v == "" {
		return fmt.Errorf("No remaining quota header: %w", ErrMissingHeaders)
	} else {
		rem, err = strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("Rate limit header is invalid: %s = %s: %v", n, v, err)
		}
	}

	if n, v := findAttr(attrs, "X-RateLimit-Reset", "ratelimit-reset"); v == "" {
		return fmt.Errorf("No window reset header: %w", ErrMissingHeaders)
	} else {
		x, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("Rate limit header is invalid: %s = %s: %v", n, v, err)
		}
		rst = l.dur.Time(x)
	}

	l.impl.Update(lim, rem, rst)

	return nil
}

func findAttr(attrs Attrs, alts ...string) (string, string) {
	for _, e := range alts {
		if v := http.Header(attrs).Get(e); v != "" {
			return e, v
		}
	}
	return "", ""
}
