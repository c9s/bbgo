package nonce

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestNonce_GetString(t *testing.T) {
	ng := NewMillisecondNonce(time.Now())

	// Generate a nonce string
	nonceStr := ng.GetString()
	if len(nonceStr) == 0 {
		t.Errorf("expected non-empty nonce string, got empty string")
	}

	// Check if the nonce string is a valid integer
	_, err := strconv.ParseInt(nonceStr, 10, 64)
	if err != nil {
		t.Errorf("expected valid integer nonce string, got error: %v", err)
	}
}

func TestNonce_GetInt64(t *testing.T) {
	ng := NewMillisecondNonce(time.Now())

	// Generate a nonce int64
	nonce := ng.GetInt64()
	if nonce <= 0 {
		t.Errorf("expected positive nonce, got %d", nonce)
	}
}

func TestNonce_Concurrency(t *testing.T) {
	ng := NewMillisecondNonce(time.Now())
	var wg sync.WaitGroup
	nonces := sync.Map{}

	// Generate nonces concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := ng.GetInt64()
			nonces.Store(n, true)
		}()
	}

	wg.Wait()

	// Check for duplicate nonces
	uniqueCount := 0
	nonces.Range(func(key, value any) bool {
		uniqueCount++
		return true
	})

	if uniqueCount != 100 {
		t.Errorf("expected 100 unique nonces, got %d", uniqueCount)
	}
}

func TestNonce_NewNonce(t *testing.T) {
	now := time.Now()
	ng := NewMillisecondNonce(now)
	if ng.current <= 0 {
		t.Errorf("expected positive initial nonce, got %d", ng.current)
	}

	// Ensure the initial nonce is based on current time
	currentTime := now.UnixMilli() * 1000
	if ng.current < currentTime {
		t.Errorf("expected initial nonce (%d) to be >= current time (%d), got %d", ng.current, currentTime, ng.current)
	}
}
