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
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 3,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		breaker.recordError(now, assert.AnError)
		assert.False(t, breaker.isHalted(now))

		breaker.recordError(now, assert.AnError)
		assert.False(t, breaker.isHalted(now))
	})

	t.Run("should halt when errors reach threshold", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 3,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)

		assert.True(t, breaker.isHalted(now))
	})

	t.Run("should auto-reset when halt duration expires", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 2,
			HaltDuration:  types.Duration(100 * time.Millisecond),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		assert.True(t, breaker.isHalted(now))

		// Check before halt duration expires - should still be halted
		assert.True(t, breaker.isHalted(now.Add(50*time.Millisecond)))

		// Check after halt duration expires - should auto-reset
		assert.False(t, breaker.isHalted(now.Add(150*time.Millisecond)))
		assert.Equal(t, 0, breaker.ErrorCount())
		assert.False(t, breaker.halted)
	})

	t.Run("should call halt callbacks only once when max error count is reached", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 2,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		breaker.SetMetricsInfo("test", "test-instance")
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
	breaker := &ErrorBreaker{
		Enabled:       true,
		MaxErrorCount: 2,
		HaltDuration:  types.Duration(time.Minute),
		ErrorWindow:   types.Duration(0),
	}
	breaker.SetMetricsInfo("test", "test-instance")
	breaker.SetMetricsInfo("test", "test-instance")
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
	breaker := &ErrorBreaker{
		Enabled:       true,
		MaxErrorCount: 5,
		HaltDuration:  types.Duration(time.Minute),
		ErrorWindow:   types.Duration(0),
	}
	breaker.SetMetricsInfo("test", "test-instance")
	breaker.SetMetricsInfo("test", "test-instance")
	now := time.Now()

	assert.Equal(t, 0, breaker.ErrorCount())

	breaker.recordError(now, assert.AnError)
	assert.Equal(t, 1, breaker.ErrorCount())

	breaker.recordError(now, assert.AnError)
	breaker.recordError(now, assert.AnError)
	assert.Equal(t, 3, breaker.ErrorCount())

	// Reset via Reset() method
	breaker.Reset()
	assert.Equal(t, 0, breaker.ErrorCount())
}

func TestErrorBreaker_ConcurrentAccess(t *testing.T) {
	breaker := &ErrorBreaker{
		Enabled:       true,
		MaxErrorCount: 20,
		HaltDuration:  types.Duration(time.Minute),
		ErrorWindow:   types.Duration(0),
	}
	breaker.SetMetricsInfo("test", "test-instance")
	breaker.SetMetricsInfo("test", "test-instance")

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
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 1,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()
		breaker.recordError(now, assert.AnError)
		assert.True(t, breaker.isHalted(now))
	})

	t.Run("very short halt duration", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 2,
			HaltDuration:  types.Duration(time.Nanosecond),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()
		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		assert.True(t, breaker.isHalted(now))
		// After a microsecond, halt should expire
		assert.False(t, breaker.isHalted(now.Add(time.Microsecond)))
	})

	t.Run("recording errors after halted state", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 2,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()
		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		assert.True(t, breaker.isHalted(now))

		// Recording more errors should keep it halted
		breaker.recordError(now, assert.AnError)
		assert.True(t, breaker.isHalted(now))
	})

	t.Run("nil error should be ignored", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 2,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		// Recording nil error adds it to internal slice, but ErrorCount filters it out
		breaker.recordError(now, nil)
		assert.Equal(t, 0, breaker.ErrorCount()) // nil is filtered out from count

		// Record a real error
		breaker.recordError(now, assert.AnError)
		assert.Equal(t, 1, breaker.ErrorCount()) // Only real error counted (nil was cleaned up)

		// Recording another nil error
		breaker.recordError(now, nil)
		assert.Equal(t, 1, breaker.ErrorCount()) // real error only, nil filtered from count

		// Record another real error (cleans up the nil)
		breaker.recordError(now, assert.AnError)
		assert.Equal(t, 2, breaker.ErrorCount()) // 2 real errors
		assert.True(t, breaker.isHalted(now))    // Halted with 2 errors (threshold is 2)
	})
}

func TestErrorBreaker_Errors(t *testing.T) {
	t.Run("should return all recorded errors", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 5,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
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
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 5,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		errors := breaker.Errors()
		assert.Empty(t, errors)
	})

	t.Run("should return empty after reset", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 5,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		breaker.recordError(now, assert.AnError)
		breaker.recordError(now, assert.AnError)
		assert.Len(t, breaker.Errors(), 2)

		breaker.Reset() // Reset via Reset() method
		assert.Empty(t, breaker.Errors())
	})

	t.Run("should not halt when error occurs outside error window", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 3,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(5 * time.Second),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		// Record 2 errors in a row
		breaker.recordError(now, assert.AnError)
		breaker.recordError(now.Add(1*time.Second), assert.AnError)
		assert.Equal(t, 2, breaker.ErrorCount())
		assert.False(t, breaker.isHalted(now.Add(1*time.Second)))

		// Third error comes in outside the error window (6 seconds after the first error)
		breaker.recordError(now.Add(6*time.Second), assert.AnError)

		// The first error should be removed (outside window), leaving 2 errors
		assert.Equal(t, 2, breaker.ErrorCount())
		assert.False(t, breaker.isHalted(now.Add(6*time.Second)))
	})

	t.Run("should halt when errors within window exceed threshold", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 3,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(10 * time.Second),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		// Record 3 errors within the window
		breaker.recordError(now, assert.AnError)
		breaker.recordError(now.Add(2*time.Second), assert.AnError)
		breaker.recordError(now.Add(4*time.Second), assert.AnError)

		// All 3 errors are within the 10-second window, should halt
		assert.Equal(t, 3, breaker.ErrorCount())
		assert.True(t, breaker.isHalted(now.Add(4*time.Second)))
	})
}

func TestErrorBreaker_Marshal(t *testing.T) {
	breaker := &ErrorBreaker{
		Enabled:       true,
		MaxErrorCount: 5,
		HaltDuration:  types.Duration(2 * time.Minute),
		ErrorWindow:   types.Duration(0),
	}
	breaker.SetMetricsInfo("test", "test-instance")

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
		assert.Equal(t, breaker.MaxErrorCount, unmarshaledBreaker.MaxErrorCount)
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
func TestErrorBreaker_WindowBasedCounting(t *testing.T) {
	t.Run("should track errors within window regardless of success between them", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 3,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(10 * time.Second),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		// Record error at T+0s
		breaker.recordError(now, assert.AnError)
		assert.Equal(t, 1, breaker.ErrorCount())

		// Record nil at T+2s (added to internal slice but filtered from count)
		breaker.recordError(now.Add(2*time.Second), nil)
		assert.Equal(t, 1, breaker.ErrorCount()) // Still 1 (nil filtered out)

		// Record error at T+4s (nil gets cleaned up from internal slice)
		breaker.recordError(now.Add(4*time.Second), assert.AnError)
		assert.Equal(t, 2, breaker.ErrorCount()) // 2 real errors

		// Record error at T+6s
		breaker.recordError(now.Add(6*time.Second), assert.AnError)
		assert.Equal(t, 3, breaker.ErrorCount())
		assert.True(t, breaker.isHalted(now.Add(6*time.Second))) // Now halted with 3 errors
	})

	t.Run("should remove errors outside window before counting", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 3,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(5 * time.Second),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		// Record error at T+0s
		breaker.recordError(now, assert.AnError)
		assert.Equal(t, 1, breaker.ErrorCount())

		// Record error at T+2s
		breaker.recordError(now.Add(2*time.Second), assert.AnError)
		assert.Equal(t, 2, breaker.ErrorCount())

		// Record error at T+8s (errors at T+0s and T+2s are now outside 5s window)
		// Window at T+8s: [T+3s, T+8s], so errors before T+3s are removed
		breaker.recordError(now.Add(8*time.Second), assert.AnError)
		// Only error at T+8s remains (errors at T+0s and T+2s removed)
		assert.Equal(t, 1, breaker.ErrorCount())
		assert.False(t, breaker.isHalted(now.Add(8*time.Second)))

		// Record error at T+10s (error at T+8s is still within window)
		// Window at T+10s: [T+5s, T+10s]
		breaker.recordError(now.Add(10*time.Second), assert.AnError)
		// Errors at T+8s and T+10s remain
		assert.Equal(t, 2, breaker.ErrorCount())
		assert.False(t, breaker.isHalted(now.Add(10*time.Second)))

		// Record error at T+11s
		// Window at T+11s: [T+6s, T+11s]
		breaker.recordError(now.Add(11*time.Second), assert.AnError)
		// Errors at T+8s, T+10s, and T+11s remain - should trigger halt
		assert.Equal(t, 3, breaker.ErrorCount())
		assert.True(t, breaker.isHalted(now.Add(11*time.Second)))
	})

	t.Run("should halt when threshold errors occur within window", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 3,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(5 * time.Second),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		// Record 3 errors within 5 second window
		breaker.recordError(now, assert.AnError)
		breaker.recordError(now.Add(1*time.Second), assert.AnError)
		breaker.recordError(now.Add(2*time.Second), assert.AnError)

		assert.Equal(t, 3, breaker.ErrorCount())
		assert.True(t, breaker.isHalted(now.Add(2*time.Second)))
	})

	t.Run("should work with zero window (all errors counted)", func(t *testing.T) {
		breaker := &ErrorBreaker{
			Enabled:       true,
			MaxErrorCount: 3,
			HaltDuration:  types.Duration(time.Minute),
			ErrorWindow:   types.Duration(0),
		}
		breaker.SetMetricsInfo("test", "test-instance")
		now := time.Now()

		// With zero window, all errors are counted regardless of timing
		breaker.recordError(now, assert.AnError)
		breaker.recordError(now.Add(100*time.Second), assert.AnError)
		breaker.recordError(now.Add(200*time.Second), assert.AnError)

		assert.Equal(t, 3, breaker.ErrorCount())
		assert.True(t, breaker.isHalted(now.Add(200*time.Second)))
	})
}
