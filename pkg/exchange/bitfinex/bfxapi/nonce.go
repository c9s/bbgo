package bfxapi

import (
	"strconv"
	"sync/atomic"
	"time"
)

type Nonce struct {
	current int64
}

// GetString generates a unique nonce based on the current time in milliseconds * 1000.
// If multiple calls occur within the same millisecond, atomic increment ensures uniqueness.
func (ng *Nonce) GetString() string {
	nonce := ng.GetInt64()
	return strconv.FormatInt(nonce, 10)
}

func (ng *Nonce) GetInt64() int64 {
	current := atomic.LoadInt64(&ng.current)
	newNonce := time.Now().UnixMilli() * 1000

	if newNonce > current {
		if atomic.CompareAndSwapInt64(&ng.current, current, newNonce) {
			return newNonce
		}
	}

	return atomic.AddInt64(&ng.current, 1)
}

func NewNonce() *Nonce {
	return &Nonce{
		current: time.Now().UnixMilli() * 1000,
	}
}
