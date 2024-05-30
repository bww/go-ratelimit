package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLinear(t *testing.T) {
	lim := NewLinear(Config{
		Start:  time.Date(2024, 4, 12, 0, 0, 0, 0, time.UTC),
		Window: time.Minute,
		Events: 6,
	})
	tests := []struct {
		When  time.Time
		Next  time.Time
		State State
		Error error
	}{
		{
			When: time.Date(2024, 4, 12, 0, 0, 0, 0, time.UTC),
			Next: time.Date(2024, 4, 12, 0, 0, 10, 0, time.UTC),
			State: State{
				Limit:     6,
				Remaining: 6,
				Reset:     time.Date(2024, 4, 12, 0, 1, 0, 0, time.UTC),
			},
		},
		{
			When: time.Date(2024, 4, 12, 0, 0, 1, 0, time.UTC),
			Next: time.Date(2024, 4, 12, 0, 0, 10, 0, time.UTC),
			State: State{
				Limit:     6,
				Remaining: 5,
				Reset:     time.Date(2024, 4, 12, 0, 1, 0, 0, time.UTC),
			},
		},
		{
			When: time.Date(2024, 4, 12, 0, 0, 9, 1000000, time.UTC),
			Next: time.Date(2024, 4, 12, 0, 0, 10, 0, time.UTC),
			State: State{
				Limit:     6,
				Remaining: 5,
				Reset:     time.Date(2024, 4, 12, 0, 1, 0, 0, time.UTC),
			},
		},
		{
			When: time.Date(2024, 4, 12, 0, 0, 59, 1000000, time.UTC),
			Next: time.Date(2024, 4, 12, 0, 1, 0, 0, time.UTC),
			State: State{
				Limit:     6,
				Remaining: 0,
				Reset:     time.Date(2024, 4, 12, 0, 1, 0, 0, time.UTC),
			},
		},
		{
			When: time.Date(2024, 4, 12, 0, 1, 0, 0, time.UTC),
			Next: time.Date(2024, 4, 12, 0, 1, 10, 0, time.UTC),
			State: State{
				Limit:     6,
				Remaining: 6,
				Reset:     time.Date(2024, 4, 12, 0, 2, 0, 0, time.UTC),
			},
		},
	}
	for i, e := range tests {
		next, err := lim.Next(e.When)
		if e.Error != nil {
			assert.ErrorIs(t, err, e.Error, "#%d", i)
		} else if assert.NoError(t, err) {
			assert.Equal(t, e.Next, next, "#%d", i)
		}
		assert.Equal(t, e.State, lim.State(e.When), "#%d", i)
	}
}
