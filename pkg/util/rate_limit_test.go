package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestNewValidRateLimiter(t *testing.T) {
	cases := []struct {
		name     string
		r        rate.Limit
		b        int
		hasError bool
	}{
		{"valid limiter", 0.1, 1, false},
		{"zero rate", 0, 1, true},
		{"zero burst", 0.1, 0, true},
		{"both zero", 0, 0, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			limiter, err := NewValidLimiter(c.r, c.b)
			assert.Equal(t, c.hasError, err != nil)
			if !c.hasError {
				assert.NotNil(t, limiter)
			}
		})
	}
}

func TestShouldDelay(t *testing.T) {
	minInterval := time.Second * 3
	maxRate := rate.Limit(1 / minInterval.Seconds())
	limiter := rate.NewLimiter(maxRate, 1)
	assert.Equal(t, time.Duration(0), ShouldDelay(limiter, minInterval))
	for i := 0; i < 100; i++ {
		assert.True(t, ShouldDelay(limiter, minInterval) > 0)
	}
}
