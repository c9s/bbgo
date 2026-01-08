package circuitbreaker

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestErrorBreaker_RecordError(t *testing.T) {
	t.Run("should not halt when errors are below threshold", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 3, types.Duration(time.Minute))
		now := time.Now()

		breaker.recordError(now, assert.AnError)
		assert.False(t, breaker.isHalted(now))

		breaker.recordError(now, assert.AnError)
		assert.False(t, breaker.isHalted(now))
	})

	t.Run("should halt when errors reach threshold", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 3, types.Duration(time.Minute))
		now := time.Now()

		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)

		assert.True(t, breaker.isHalted(now))
	})

	t.Run("should reset when nil error is recorded", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 3, types.Duration(time.Minute))
		now := time.Now()

		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		assert.Equal(t, 2, breaker.ErrorCount())

		// Recording nil error should reset the breaker
		breaker.recordError(now, nil)
		assert.False(t, breaker.isHalted(now))
		assert.Equal(t, 0, breaker.ErrorCount())
	})

	t.Run("should auto-reset when halt duration expires", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 2, types.Duration(100*time.Millisecond))
		now := time.Now()

		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		assert.True(t, breaker.isHalted(now))

		// Check before halt duration expires - should still be halted
		assert.True(t, breaker.isHalted(now.Add(50*time.Millisecond)))

		// Check after halt duration expires - should auto-reset
		assert.False(t, breaker.isHalted(now.Add(150*time.Millisecond)))
		assert.Equal(t, 0, breaker.ErrorCount())
	})

	t.Run("should call halt callbacks only once when max error count is reached", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 2, types.Duration(time.Minute))
		now := time.Now()

		// Track callback invocations
		callbackCalled := false
		callCount := 0
		var callbackTime time.Time

		breaker.OnHalt(func(t time.Time, records []ErrorRecord) {
			callbackCalled = true
			callCount++
			callbackTime = t
		})

		// Record first error - callback should not be called yet
		breaker.recordError(now, assert.AnError)
		assert.False(t, callbackCalled, "callback should not be called before threshold")
		assert.Equal(t, 0, callCount)

		// Record second error to reach threshold - callback should be called once
		breaker.recordError(now, assert.AnError)
		assert.True(t, callbackCalled, "halt callback should have been called")
		assert.Equal(t, 1, callCount)
		assert.Equal(t, now, callbackTime)

		// Check if halted - callback should not be called again
		assert.True(t, breaker.isHalted(now))
		assert.Equal(t, 1, callCount, "callback should only be called once")

		// Record more errors - callback should still not be called again
		breaker.recordError(now, assert.AnError)
		assert.Equal(t, 1, callCount, "callback should only be called once even with more errors")
	})
}

func TestErrorBreaker_Reset(t *testing.T) {
	breaker := NewErrorBreaker("test", "test-instance", 2, types.Duration(time.Minute))
	now := time.Now()

	breaker.recordError(now, assert.AnError)
	breaker.recordError(now, assert.AnError)

	assert.True(t, breaker.isHalted(now))
	assert.Equal(t, 2, breaker.ErrorCount())

	breaker.Reset()

	assert.False(t, breaker.isHalted(now))
	assert.Equal(t, 0, breaker.ErrorCount())
}

func TestErrorBreaker_ErrorCount(t *testing.T) {
	breaker := NewErrorBreaker("test", "test-instance", 5, types.Duration(time.Minute))
	now := time.Now()

	assert.Equal(t, 0, breaker.ErrorCount())

	breaker.recordError(now, assert.AnError)
	assert.Equal(t, 1, breaker.ErrorCount())

	breaker.recordError(now, assert.AnError)
	breaker.recordError(now, assert.AnError)
	assert.Equal(t, 3, breaker.ErrorCount())

	// Reset via nil error
	breaker.recordError(now, nil)
	assert.Equal(t, 0, breaker.ErrorCount())
}

func TestErrorBreaker_ConcurrentAccess(t *testing.T) {
	breaker := NewErrorBreaker("test", "test-instance", 20, types.Duration(time.Minute))

	// Spawn multiple goroutines to record errors concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			for j := 0; j < 4; j++ {
				breaker.RecordError(assert.AnError)
			}
		}(&wg)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	now := time.Now()
	assert.True(t, breaker.isHalted(now.Add(time.Second*10)))
	assert.Equal(t, 20, breaker.ErrorCount())
}

func TestErrorBreaker_EdgeCases(t *testing.T) {
	t.Run("maxErrors of 1 should halt immediately", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 1, types.Duration(time.Minute))
		now := time.Now()
		breaker.recordError(now, assert.AnError)
		assert.True(t, breaker.isHalted(now))
	})

	t.Run("very short halt duration", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 2, types.Duration(time.Nanosecond))
		now := time.Now()
		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		assert.True(t, breaker.isHalted(now))
		// After a microsecond, halt should expire
		assert.False(t, breaker.isHalted(now.Add(time.Microsecond)))
	})

	t.Run("recording errors after halted state", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 2, types.Duration(time.Minute))
		now := time.Now()
		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		assert.True(t, breaker.isHalted(now))

		// Recording more errors should keep it halted
		breaker.recordError(now, assert.AnError)
		assert.True(t, breaker.isHalted(now))
	})

	t.Run("nil error should reset breaker", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 2, types.Duration(time.Minute))
		now := time.Now()

		breaker.recordError(now, nil)
		assert.False(t, breaker.isHalted(now))
		assert.Equal(t, 0, breaker.ErrorCount())

		// Recording real error then nil should reset
		breaker.recordError(now, assert.AnError)
		assert.Equal(t, 1, breaker.ErrorCount())
		breaker.recordError(now, nil) // Should reset
		assert.Equal(t, 0, breaker.ErrorCount())
		assert.False(t, breaker.isHalted(now))
	})
}

func TestErrorBreaker_Errors(t *testing.T) {
	t.Run("should return all recorded errors", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 5, types.Duration(time.Minute))
		now := time.Now()

		err1 := assert.AnError
		err2 := assert.AnError
		err3 := assert.AnError

		breaker.recordError(now, err1)
		breaker.recordError(now, err2)
		breaker.recordError(now, err3)

		errors := breaker.Errors()
		assert.Len(t, errors, 3)
	})

	t.Run("should return empty slice when no errors", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 5, types.Duration(time.Minute))
		errors := breaker.Errors()
		assert.Empty(t, errors)
	})

	t.Run("should return empty after reset", func(t *testing.T) {
		breaker := NewErrorBreaker("test", "test-instance", 5, types.Duration(time.Minute))
		now := time.Now()

		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		assert.Len(t, breaker.Errors(), 2)

		breaker.recordError(now, nil) // Reset via nil error
		assert.Empty(t, breaker.Errors())
	})
}

func TestErrorBreaker_Marshal(t *testing.T) {
	breaker := NewErrorBreaker("test-strategy", "test-instance", 5, types.Duration(2*time.Minute))

	data, err := json.Marshal(breaker)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Step 3: Unmarshal the breaker back
	var unmarshaledBreaker ErrorBreaker
	err = json.Unmarshal(data, &unmarshaledBreaker)
	assert.NoError(t, err)

	unmarshaledBreaker.SetMetricsInfo("test-strategy", "test-instance")

	// The unmarshaled breaker should be treated as reset
	t.Run("verify configuration is preserved", func(t *testing.T) {
		assert.Equal(t, breaker.MaxConsecutiveErrorCount, unmarshaledBreaker.MaxConsecutiveErrorCount)
		assert.Equal(t, breaker.HaltDuration, unmarshaledBreaker.HaltDuration)
		assert.Equal(t, breaker.strategyInstance, unmarshaledBreaker.strategyInstance)
	})

	t.Run("verify breaker starts in reset state", func(t *testing.T) {
		// After unmarshaling, breaker should be treated as reset
		assert.False(t, unmarshaledBreaker.IsHalted())
	})

	t.Run("verify RecordError functionality", func(t *testing.T) {
		// Add errors to reach threshold (5 errors)
		unmarshaledBreaker.RecordError(assert.AnError)
		assert.Equal(t, 1, unmarshaledBreaker.ErrorCount())
		assert.False(t, unmarshaledBreaker.IsHalted())

		unmarshaledBreaker.RecordError(assert.AnError)
		unmarshaledBreaker.RecordError(assert.AnError)
		unmarshaledBreaker.RecordError(assert.AnError)
		assert.Equal(t, 4, unmarshaledBreaker.ErrorCount())
		assert.False(t, unmarshaledBreaker.IsHalted())

		// 5th error should trigger halt
		unmarshaledBreaker.RecordError(assert.AnError)
		assert.Equal(t, 5, unmarshaledBreaker.ErrorCount())
		assert.True(t, unmarshaledBreaker.IsHalted())
	})

	t.Run("verify Reset functionality", func(t *testing.T) {
		// Reset the breaker
		unmarshaledBreaker.Reset()
		assert.Equal(t, 0, unmarshaledBreaker.ErrorCount())
		assert.False(t, unmarshaledBreaker.IsHalted())
		assert.Empty(t, unmarshaledBreaker.Errors())

		unmarshaledBreaker.RecordError(assert.AnError)
		unmarshaledBreaker.RecordError(assert.AnError)

		errors := unmarshaledBreaker.Errors()
		assert.NotEmpty(t, errors)
		for _, err := range errors {
			assert.NotNil(t, err)
		}
		assert.Equal(t, 2, len(errors))
	})
}
