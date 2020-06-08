package util

import (
	"fmt"
	"time"

	"golang.org/x/time/rate"
)

func ShouldDelay(l *rate.Limiter, minInterval time.Duration) time.Duration {
	return l.Reserve().Delay()
}

func NewValidLimiter(r rate.Limit, b int) (*rate.Limiter, error) {
	if b <= 0 || r <= 0 {
		return nil, fmt.Errorf("Bad rate limit config. Insufficient tokens. (rate=%f, b=%d)", r, b)
	}
	return rate.NewLimiter(r, b), nil
}
