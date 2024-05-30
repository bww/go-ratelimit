package ratelimit

import (
	"sync"
	"time"
)

const (
	lowThreshold = 0.05  // quota is running low when we have 5% of operations remaining
	lowLimit     = 0.005 // stop making requests when we only have ½% of operations left
)

const defaultBackoffPeriod = time.Minute * 3

// Compute the backoff duration for a period and error count
func backoffDuration(p time.Duration, n int) time.Duration {
	return p * time.Duration(n) * time.Duration(n)
}

// limiter implements the basic mechanics of a rate limiter, but it does not
// conform to RateLimiter and its state must be updated explicitly, rather than
// from an HTTP response. It is intended to be used as a basis for other rate
// limiters which provide the service rate limit state but which do not need to
// reimplement the basic rate limiting logic.
type limiter struct {
	sync.Mutex
	limit         int
	remaining     int
	reset         time.Time
	backoff       *time.Time
	backoffPeriod time.Duration
	errcount      int
	mode          Mode
	target        float64       // the proprortion of the total quota we target, if > 0
	maxMeter      time.Duration // maximum delay in metered mode, if > 0
}

func newLimiter(q, r int, e time.Time) limiter {
	return limiter{
		limit:         q,
		remaining:     r,
		reset:         e,
		backoffPeriod: defaultBackoffPeriod,
	}
}

func (l *limiter) SetMode(m Mode) {
	l.Lock()
	defer l.Unlock()
	l.mode = m
}

func (l *limiter) SetMaxMeterDelay(d time.Duration) {
	l.Lock()
	defer l.Unlock()
	l.maxMeter = d
}

func (l *limiter) SetTarget(t float64) {
	l.Lock()
	defer l.Unlock()
	if t > 1 {
		l.target = 1
	} else if t < 0 {
		l.target = 0
	} else {
		l.target = t
	}
}

func (l *limiter) State() State {
	l.Lock()
	defer l.Unlock()
	return State{
		Limit:     l.limit,
		Remaining: l.remaining,
		Reset:     l.reset,
	}
}

// Update remaining budget to the provided state
func (l *limiter) Update(lim, rem int, rst time.Time) error {
	l.Lock()
	defer l.Unlock()
	l.limit = lim
	l.remaining = rem
	l.reset = rst
	return nil
}

// Decrement remaining budget if we have any
func (l *limiter) Dec() error {
	l.Lock()
	defer l.Unlock()
	if l.remaining > 0 {
		l.remaining--
	}
	return nil
}

// Back off incrementally, relative to the provided time
func (l *limiter) Backoff(rel time.Time) (time.Time, error) {
	l.Lock()
	defer l.Unlock()
	l.errcount++
	until := rel.Add(backoffDuration(l.backoffPeriod, l.errcount))
	l.backoff = &until
	return until, nil
}

// Back off until the provided time
func (l *limiter) BackoffUntil(until time.Time) error {
	l.Lock()
	defer l.Unlock()
	l.backoff = &until
	l.errcount = 1
	return nil
}

// Invalidate a backoff period
func (l *limiter) InvalidateBackoff() error {
	l.Lock()
	defer l.Unlock()
	l.errcount = 0
	l.backoff = nil
	return nil
}

func (l *limiter) Delay(rel time.Time) (time.Duration, error) {
	var (
		d, r time.Duration
		b    *time.Time
		m    Mode
		q, e int
	)

	// mutate state in one chunk
	l.Lock()
	m = l.mode
	q = l.limit

	// first, check for an existing backoff period
	if v := l.backoff; v != nil {
		if rel.After(*v) {
			l.backoff = nil
		} else {
			b = v
		}
	}

	// if we don't have one, determine if we have budget left, and if so
	// consume a request; otherwise, the delay is until the window reset
	if b == nil {
		r = l.reset.Sub(rel)
		if r < 0 {
			r = 0 // can't have a negative reset window
		}
		e = l.remaining
		if l.remaining > 0 {
			l.remaining--
		} else {
			d = r
		}
		l.errcount = 0 // clear error count if we're not in a backoff
	}

	l.Unlock()

	// if we are in a backoff, the delay is until the backoff period ends
	if b != nil {
		return (*b).Sub(rel), nil
	}
	// if we have exhausted the current window, the delay is the end of the window
	if d > 0 {
		return d, nil
	}

	// if we are using Meter mode, we attempt to spread out our requests over
	// the entire rate-limit window rather than consuming them until we exhaust
	// the budget and then waiting for the window to reset
	if m == Meter && e > 0 {
		d := r / time.Duration(e)
		if l.target > 0 {
			d = time.Duration(float64(d) * (1.0 / l.target))
		}
		// back off aggressively as we get close to our limit
		if p := float64(e) / float64(q); p < lowLimit {
			d = r // wait until the window resets
		} else if p < lowThreshold {
			d = time.Duration(float64(d) * (1.0 / p / 2.0))
		}
		if x := l.maxMeter; x > 0 && d > x {
			return x, nil
		} else {
			return d, nil
		}
	}

	return 0, nil
}
