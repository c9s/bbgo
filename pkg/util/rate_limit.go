package util

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
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

// ParseRateLimitSyntax parses the rate limit syntax into the rate.Limiter parameters
// sample inputs:
//
//		 2+1/5s (2 initial tokens, 1 token per 5 seconds)
//		 5+3/1m (5 initial tokens, 3 tokens per minute)
//	     3m (3 tokens per minute)
//	     1/3m (1 token per 3 minutes)
func ParseRateLimitSyntax(desc string) (*rate.Limiter, error) {
	var b = 0
	var r = 1.0
	var durStr string
	var duration time.Duration

	_, err := fmt.Sscanf(desc, "%d+%f/%s", &b, &r, &durStr)
	if err != nil {
		b = 0
		r = 1.0
		_, err = fmt.Sscanf(desc, "%f/%s", &r, &durStr)
		if err != nil {
			durStr = desc
			// need to reset
			b = 1
			r = 1.0
		}
	}

	duration, err = time.ParseDuration(durStr)
	if err != nil {
		return nil, fmt.Errorf("invalid rate limit syntax: b+n/duration, err: %v", err)
	}
	if r == 1.0 {
		return NewValidLimiter(rate.Every(duration), b)
	}

	log.Infof("%v %v", duration, r)
	return NewValidLimiter(rate.Every(time.Duration(float64(duration)/r)), b)
}
