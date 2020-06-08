package websocket

import (
	"time"
)

const (
	DefaultMinBackoff = 5 * time.Second
	DefaultMaxBackoff = time.Minute

	DefaultBackoffFactor = 2
)

type Backoff struct {
	attempt       int64
	Factor        float64
	cur, Min, Max time.Duration
}

// Duration returns the duration for the current attempt before incrementing
// the attempt counter. See ForAttempt.
func (b *Backoff) Duration() time.Duration {
	d := b.calculate(b.attempt)
	b.attempt++
	return d
}

func (b *Backoff) calculate(attempt int64) time.Duration {
	min := b.Min
	if min <= 0 {
		min = DefaultMinBackoff
	}
	max := b.Max
	if max <= 0 {
		max = DefaultMaxBackoff
	}
	if min >= max {
		return max
	}
	factor := b.Factor
	if factor <= 0 {
		factor = DefaultBackoffFactor
	}
	cur := b.cur
	if cur < min {
		cur = min
	} else if cur > max {
		cur = max
	}

	//calculate this duration
	next := cur
	if attempt > 0 {
		next = time.Duration(float64(cur) * factor)
	}

	if next < cur {
		// overflow
		next = max
	} else if next <= min {
		next = min
	} else if next >= max {
		next = max
	}
	b.cur = next
	return next
}

// Reset restarts the current attempt counter at zero.
func (b *Backoff) Reset() {
	b.attempt = 0
	b.cur = b.Min
}

// Attempt returns the current attempt counter value.
func (b *Backoff) Attempt() int64 {
	return b.attempt
}
