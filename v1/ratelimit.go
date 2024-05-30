package ratelimit

import (
	"context"
	"net/http"
	"time"
)

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

// Derive rate limiting attributes from a HTTP request
func AttrsFromResponse(rsp *http.Response) Attrs {
	return Attrs(rsp.Header)
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

// WithResponse is a convenience function which derives attributes from the
// provided response and then applies them to the options. It is the equivalent
// of:
//
//	WithAttrs(AttrsFromResponse(req))
func WithResponse(v *http.Response) Option {
	return WithAttrs(AttrsFromResponse(v))
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
	// Wait blocks until the next request can be executed.
	Wait(context.Context, time.Time) (time.Time, error)
	// Update provides post-operation feedback to the rate limiter. An implementation may use this context or not.
	Update(time.Time, ...Option) error
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
