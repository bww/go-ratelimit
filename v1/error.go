package ratelimit

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrCanceled       = errors.New("Canceled")
	ErrMissingHeaders = errors.New("Missing rate-limiting headers")
)

// RetryError represents a rate limiting error from a remote service that
// indicates when we should attempt our operation again.
type RetryError struct {
	Cause      error
	RetryAfter time.Time
}

func (e RetryError) Unwrap() error {
	return e.Cause
}

func (e RetryError) Error() string {
	if c := e.Cause; c != nil {
		return c.Error()
	} else {
		return fmt.Sprintf("Retry after: %v", e.RetryAfter)
	}
}
