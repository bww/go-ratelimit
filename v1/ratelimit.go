package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"time"
)

var ErrCanceled = errors.New("Canceled")

// A snapshot of a Limiter's state
type State struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

// Attributes which may be factored into rate limiting implementations
type Attrs map[string][]string

// Derive rate limiting attributes from a HTTP request
func AttrsFromRequest(req *http.Request) Attrs {
	return Attrs(req.Header)
}

// Options provides addional contextual details to a rate limiter
type Options struct {
	Attrs Attrs
}

// With applies additional options to the receiver
func (c Options) With(opts []Option) Options {
	for _, opt := range opts {
		c = opt(c)
	}
	return c
}

// A functional option
type Option func(Options) Options

// WithRequest is a convenience function which derives attributes from the
// provided request and then applies them to the options. It is the equivalent
// of:
//
//	WithAttrs(AttrsFromRequest(req))
func WithRequest(v *http.Request) Option {
	return WithAttrs(AttrsFromRequest(v))
}

// WithAttrs adds attributes to a set of options
func WithAttrs(v Attrs) Option {
	return func(c Options) Options {
		c.Attrs = v
		return c
	}
}

// A general purpose rate limiter
type Limiter interface {
	// Next returns the time at which the next request can be executed relative to the provided time.
	Next(time.Time, ...Option) (time.Time, error)
	// Wait blocks until the next request can be executed
	Wait(context.Context, time.Time) (time.Time, error)
	// State provides a snapshot of the rate limiter's general state. Not all implementations can fully describe this state.
	State(time.Time) State
}

// General rate limiting configuration
type Config struct {
	// The initial base window reference time
	Start time.Time
	// The duration of a window: this is the period over which we limit the number of requests
	Window time.Duration
	// The number of events permitted within a single window
	Events int
}

// Linear implements a rate limiter which spreads out requests evenly
// over the window period.
type Linear struct {
	Config
	base  time.Time
	delay time.Duration
}

func NewLinear(conf Config) *Linear {
	var when time.Time
	if !conf.Start.IsZero() {
		when = conf.Start
	} else {
		when = time.Now()
	}
	return &Linear{
		Config: conf,
		base:   when,
		delay:  conf.Window / time.Duration(conf.Events),
	}
}

func (l *Linear) State(rel time.Time) State {
	var (
		nwin  = rel.Sub(l.base) / l.Window
		start = l.base.Add(nwin * l.Window)
		reset = start.Add(l.Window)
		curr  = rel.Sub(start)
	)
	return State{
		Limit:     l.Events,
		Remaining: int((1 - (float64(curr) / float64(l.Window))) * float64(l.Events)),
		Reset:     reset,
	}
}

func (l *Linear) Next(rel time.Time, opts ...Option) (time.Time, error) {
	dm := int64(l.delay / 1000)
	return time.UnixMicro(((rel.UnixMicro() / dm) * dm) + int64(l.delay/1000)).UTC(), nil
}

func (l *Linear) Wait(cxt context.Context, rel time.Time, opts ...Option) (time.Time, error) {
	t, err := l.Next(rel, opts...)
	if err != nil {
		return time.Time{}, err
	}
	select {
	case <-time.After(t.Sub(rel)):
		return t, nil
	case <-cxt.Done():
		return t, ErrCanceled
	}
}
