package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLinear(t *testing.T) {
	lim := NewLinear(Config{
		Window: time.Minute,
		Events: 6,
	})
	tests := []struct {
		When time.Time
		Next time.Time
		Wait time.Duration
	}{
		{
			When: time.Date(2024, 4, 12, 0, 0, 0, 0, time.UTC),
			Next: time.Date(2024, 4, 12, 0, 0, 10, 0, time.UTC),
		},
		{
			When: time.Date(2024, 4, 12, 0, 0, 1, 0, time.UTC),
			Next: time.Date(2024, 4, 12, 0, 0, 10, 0, time.UTC),
		},
		{
			When: time.Date(2024, 4, 12, 0, 0, 9, 1000000, time.UTC),
			Next: time.Date(2024, 4, 12, 0, 0, 10, 0, time.UTC),
		},
		{
			When: time.Date(2024, 4, 12, 0, 0, 9, 1000000, time.UTC),
			Next: time.Date(2024, 4, 12, 0, 0, 10, 0, time.UTC),
		},
	}
	for _, e := range tests {
		assert.Equal(t, e.Next, lim.Next(e.When))
	}
}
