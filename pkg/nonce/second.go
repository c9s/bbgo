package nonce

import (
	"strconv"
	"sync/atomic"
	"time"
)

// SecondNonce generates nonce values based on the current time in seconds with microsecond precision.
type SecondNonce struct {
	current int64
}

// GetString generates a unique nonce based on the current time in seconds * 1000.
func (ng *SecondNonce) GetString() string {
	nonce := ng.GetInt64()
	return strconv.FormatInt(nonce, 10)
}

// GetInt64 generates a unique nonce based on the current time in seconds * 1000.
// If multiple calls occur within the same second, atomic increment ensures uniqueness.
// The unit is millisecond (1 second = 1,000 milliseconds).
func (ng *SecondNonce) GetInt64() int64 {
	current := atomic.LoadInt64(&ng.current)
	newNonce := time.Now().Unix() * 1000

	if newNonce > current {
		if atomic.CompareAndSwapInt64(&ng.current, current, newNonce) {
			return newNonce
		}
	}

	return atomic.AddInt64(&ng.current, 1)
}

func NewSecondNonce(now time.Time) *SecondNonce {
	return &SecondNonce{
		current: now.Unix() * 1000,
	}
}
