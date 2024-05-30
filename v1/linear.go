package ratelimit

import (
	"context"
	"time"
)

// linear implements a rate limiter which spreads out requests evenly
// over the window period.
type linear struct {
	Config
	base  time.Time
	delay time.Duration
}

func NewLinear(conf Config) *linear {
	var when time.Time
	if !conf.Start.IsZero() {
		when = conf.Start
	} else {
		when = time.Now()
	}
	return &linear{
		Config: conf,
		base:   when,
		delay:  conf.Window / time.Duration(conf.Events),
	}
}

func (l *linear) State(rel time.Time) State {
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

func (l *linear) Next(rel time.Time, opts ...Option) (time.Time, error) {
	dm := int64(l.delay / 1000)
	return time.UnixMicro(((rel.UnixMicro() / dm) * dm) + int64(l.delay/1000)).UTC(), nil
}

func (l *linear) Wait(cxt context.Context, rel time.Time, opts ...Option) (time.Time, error) {
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

func (l *linear) Update(rel time.Time, opts ...Option) error {
	// Linear implementation does not use post-operation state
	return nil
}
