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

func TestParseRateLimitSyntax(t *testing.T) {
	var tests = []struct {
		desc          string
		expectedBurst int
		expectedRate  float64

		wantErr bool
	}{
		{"2+1/5s", 2, 1.0 / 5.0, false},
		{"5+1/3m", 5, 1 / 3 * 60.0, false},
		{"1m", 1, 1.0 / 60.0, false},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			limiter, err := ParseRateLimitSyntax(test.desc)
			if test.wantErr {
				assert.Error(t, err)
			} else if assert.NoError(t, err) {
				assert.NotNil(t, limiter)
				burst := limiter.Burst()
				assert.Equal(t, test.expectedBurst, burst)

				limit := limiter.Limit()
				assert.InDeltaf(t, test.expectedRate, float64(limit), 0.01, "expected rate %f, got %f", test.expectedRate, limit)
			}
		})
	}
}
