package ratelimit

import (
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
type headersLimiter struct {
	impl limiter
	dur  Durationer
}

func NewHeaders(conf Config) *headersLimiter {
	var dur Durationer
	if d := conf.Durationer; d != nil {
		dur = d
	} else {
		dur = Seconds
	}
	return &headersLimiter{
		impl: newLimiter(conf.Events, conf.Events, ext.Coalesce(conf.Start, time.Now())),
		dur:  dur,
	}
}

func (l *headersLimiter) Update(rsp *http.Response) error {
	var lim, rem int
	var rst time.Time
	var err error

	// retry-after may be present even when other rate limit headers are not, handle it first
	if n, v := findHeader(rsp, "X-Retry-After", "Retry-After"); v != "" {
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

	if n, v := findHeader(rsp, "X-RateLimit-Limit", "ratelimit-limit"); v == "" {
		return fmt.Errorf("No quota limit header: %w", ErrMissingHeaders)
	} else {
		lim, err = strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("Rate limit header is invalid: %s = %s: %v", n, v, err)
		}
	}

	if n, v := findHeader(rsp, "X-RateLimit-Remaining", "ratelimit-remaining"); v == "" {
		return fmt.Errorf("No remaining quota header: %w", ErrMissingHeaders)
	} else {
		rem, err = strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("Rate limit header is invalid: %s = %s: %v", n, v, err)
		}
	}

	if n, v := findHeader(rsp, "X-RateLimit-Reset", "ratelimit-reset"); v == "" {
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

func findHeader(rsp *http.Response, alts ...string) (string, string) {
	for _, e := range alts {
		if v := rsp.Header.Get(e); v != "" {
			return e, v
		}
	}
	return "", ""
}
