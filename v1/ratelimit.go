package ratelimit

import (
	"context"
	"errors"
	"time"
)

var ErrCanceled = errors.New("Canceled")

// A general purpose rate limiter
type Limiter interface {
	// Next returns the time at which the next request can be executed relative to the provided time
	Next(time.Time) time.Time
	// Wait blocks until the next request can be executed
	Wait(context.Context, time.Time) (time.Time, error)
}

// General rate limiting configuration
type Config struct {
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
	reset time.Time
	delay time.Duration
	n     int
}

func NewLinear(conf Config) *Linear {
	now := time.Now()
	return &Linear{
		Config: conf,
		base:   now,
		reset:  now.Add(conf.Window),
		delay:  conf.Window / time.Duration(conf.Events),
		n:      0,
	}
}

func (l *Linear) Next(rel time.Time) time.Time {
	dm := int64(l.delay / 1000)
	return time.UnixMicro(((rel.UnixMicro() / dm) * dm) + int64(l.delay/1000)).UTC()
}

func (l *Linear) Wait(cxt context.Context, rel time.Time) (time.Time, error) {
	t := l.Next(rel)
	select {
	case <-time.After(t.Sub(rel)):
		return t, nil
	case <-cxt.Done():
		return t, ErrCanceled
	}
}
