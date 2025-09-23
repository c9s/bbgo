package nonce

import (
	"strconv"
	"sync/atomic"
	"time"
)

// MillisecondNonce generates nonce values based on the current time in milliseconds with microsecond precision.
type MillisecondNonce struct {
	current int64
}

// GetString generates a unique nonce based on the current time in milliseconds * 1000.
// If multiple calls occur within the same millisecond, atomic increment ensures uniqueness.
func (ng *MillisecondNonce) GetString() string {
	nonce := ng.GetInt64()
	return strconv.FormatInt(nonce, 10)
}

// GetInt64 generates a unique nonce based on the current time in milliseconds * 1000.
// If multiple calls occur within the same millisecond, atomic increment ensures uniqueness.
// The unit is microsecond (1 second = 1,000,000 microseconds).
func (ng *MillisecondNonce) GetInt64() int64 {
	current := atomic.LoadInt64(&ng.current)
	newNonce := time.Now().UnixMilli() * 1000

	if newNonce > current {
		if atomic.CompareAndSwapInt64(&ng.current, current, newNonce) {
			return newNonce
		}
	}

	return atomic.AddInt64(&ng.current, 1)
}

func NewMillisecondNonce(now time.Time) *MillisecondNonce {
	return &MillisecondNonce{
		current: now.UnixMilli() * 1000,
	}
}
