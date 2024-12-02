package detector

import (
	"math"
	"sync"
	"time"
)

type Record[T any] struct {
	Value T
	Time  time.Time
}

type DeviationDetector[T any] struct {
	mu            sync.Mutex
	expectedValue T               // Expected value for comparison
	tolerance     float64         // Tolerance percentage (e.g., 0.01 for 1%)
	duration      time.Duration   // Time limit for sustained deviation
	toFloat64     func(T) float64 // Function to convert T to float64
	records       []Record[T]     // Tracks deviation records
}

// NewDeviationDetector creates a new instance of DeviationDetector
func NewDeviationDetector[T any](
	expectedValue T, tolerance float64, duration time.Duration, toFloat64 func(T) float64,
) *DeviationDetector[T] {
	if toFloat64 == nil {
		if _, ok := any(expectedValue).(float64); ok {
			toFloat64 = func(value T) float64 {
				return any(value).(float64)
			}
		} else {
			panic("No conversion function provided for non-float64 type")
		}
	}

	return &DeviationDetector[T]{
		expectedValue: expectedValue,
		tolerance:     tolerance,
		duration:      duration,
		toFloat64:     toFloat64,
		records:       nil,
	}
}

func (d *DeviationDetector[T]) AddRecord(value T, at time.Time) (bool, time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Calculate deviation percentage
	expected := d.toFloat64(d.expectedValue)
	current := d.toFloat64(value)
	deviationPercentage := math.Abs((current - expected) / expected)

	// Reset records if deviation is within tolerance
	if deviationPercentage <= d.tolerance {
		d.records = nil
		return false, 0
	}

	// If deviation exceeds tolerance, track the record
	if len(d.records) == 0 {
		// No prior deviation, start tracking
		d.records = []Record[T]{{Value: value, Time: at}}
		return false, 0
	}

	// Append new record
	d.records = append(d.records, Record[T]{Value: value, Time: at})

	// Calculate the sustained duration
	firstRecord := d.records[0]
	sustainedDuration := at.Sub(firstRecord.Time)
	return sustainedDuration >= d.duration, sustainedDuration
}

// GetRecords retrieves all deviation records
func (d *DeviationDetector[T]) GetRecords() []Record[T] {
	d.mu.Lock()
	defer d.mu.Unlock()

	return append([]Record[T](nil), d.records...) // Return a copy of the records
}

// ClearRecords clears all deviation records
func (d *DeviationDetector[T]) ClearRecords() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.records = nil
}
