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
	records       map[string][]Record[T] // Stores records for different keys
	expectedValue T                      // Expected value for comparison
	tolerance     float64                // Tolerance percentage (e.g., 0.01 for 1%)
	duration      time.Duration          // Time limit for sustained deviation
	toFloat64     func(T) float64        // Function to convert T to float64
}

// NewDeviationDetector creates a new instance of DeviationDetector
func NewDeviationDetector[T any](
	expectedValue T, tolerance float64, duration time.Duration, toFloat64 func(T) float64,
) *DeviationDetector[T] {
	// If no conversion function is provided and T is float64, use the default converter
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
		records:       make(map[string][]Record[T]),
		expectedValue: expectedValue,
		tolerance:     tolerance,
		duration:      duration,
		toFloat64:     toFloat64,
	}
}

// AddRecord adds a new record and checks deviation status
func (d *DeviationDetector[T]) AddRecord(key string, value T, at time.Time) (bool, time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Calculate deviation percentage
	expected := d.toFloat64(d.expectedValue)
	current := d.toFloat64(value)
	deviationPercentage := math.Abs((current - expected) / expected)

	// Reset records if deviation is within tolerance
	if deviationPercentage <= d.tolerance {
		delete(d.records, key)
		return false, 0
	}

	// If deviation exceeds tolerance, track the record
	records, exists := d.records[key]
	if !exists {
		// No prior deviation, start tracking
		d.records[key] = []Record[T]{{Value: value, Time: at}}
		return false, 0
	}

	// If deviation already being tracked, append the new record
	d.records[key] = append(records, Record[T]{Value: value, Time: at})

	// Calculate the duration of sustained deviation
	firstRecord := records[0]
	sustainedDuration := at.Sub(firstRecord.Time)
	return sustainedDuration >= d.duration, sustainedDuration
}

// GetRecords retrieves all records associated with the specified key
func (d *DeviationDetector[T]) GetRecords(key string) []Record[T] {
	d.mu.Lock()
	defer d.mu.Unlock()

	if records, exists := d.records[key]; exists {
		return records
	}
	return nil
}

// ClearRecords removes all records associated with the specified key
func (d *DeviationDetector[T]) ClearRecords(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.records, key)
}

// PruneOldRecords removes records that are older than the specified duration
func (d *DeviationDetector[T]) PruneOldRecords(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, records := range d.records {
		prunedRecords := make([]Record[T], 0)
		for _, record := range records {
			if now.Sub(record.Time) <= d.duration {
				prunedRecords = append(prunedRecords, record)
			}
		}
		d.records[key] = prunedRecords
	}
}
