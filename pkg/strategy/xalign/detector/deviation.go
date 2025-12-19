package detector

import (
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Record[T any] struct {
	Value T
	Time  time.Time
}

type DeviationDetector[T any] struct {
	mu            sync.Mutex
	expectedValue T             // Expected value for comparison
	tolerance     float64       // Tolerance percentage (e.g., 0.01 for 1%)
	duration      time.Duration // Time limit for sustained deviation

	toFloat64Amount func(T) (float64, error) // Function to convert T to float64
	records         []Record[T]              // Tracks deviation records

	logger logrus.FieldLogger
}

// NewDeviationDetector creates a new instance of DeviationDetector
func NewDeviationDetector[T any](
	expectedValue T, tolerance float64, duration time.Duration, converter func(T) (float64, error),
) *DeviationDetector[T] {
	if converter == nil {
		if _, ok := any(expectedValue).(float64); ok {
			// create default converter
			converter = func(value T) (float64, error) {
				return any(value).(float64), nil
			}
		} else {
			panic("No conversion function provided for non-float64 type")
		}
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	return &DeviationDetector[T]{
		expectedValue:   expectedValue,
		tolerance:       tolerance,
		duration:        duration,
		toFloat64Amount: converter,
		records:         nil,
		logger:          logger,
	}
}

func (d *DeviationDetector[T]) SetLogger(logger logrus.FieldLogger) {
	d.logger = logger
}

func (d *DeviationDetector[T]) SetDuration(duration time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.duration = duration
}

func (d *DeviationDetector[T]) GetDuration() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.duration
}

func (d *DeviationDetector[T]) AddRecord(at time.Time, value T) (bool, time.Duration) {
	// Calculate deviation percentage
	expected, err := d.toFloat64Amount(d.expectedValue)
	if err != nil {
		d.logger.WithError(err).Errorf("unable to calculate deviation for value %v", value)
		return false, 0
	}

	current, err := d.toFloat64Amount(value)
	if err != nil {
		d.logger.WithError(err).Errorf("unable to calculate deviation for value %v", value)
		return false, 0
	}

	deviationPercentage := math.Abs((current - expected) / expected)

	d.logger.Infof("deviation detection: current/expected=%f/%f, deviation=%.2f%%", current, expected, deviationPercentage*100.0)

	d.mu.Lock()
	defer d.mu.Unlock()

	// Reset records if deviation is within tolerance
	if deviationPercentage <= d.tolerance {
		if len(d.records) > 0 {
			d.logger.Infof("deviation within tolerance, resetting records")
		}

		d.records = nil
		return false, 0
	}

	record := Record[T]{Value: value, Time: at}

	// If deviation exceeds tolerance, track the record
	if len(d.records) == 0 {
		// No prior deviation, start tracking
		d.records = []Record[T]{record}
		return false, 0
	}

	// Append new record
	d.records = append(d.records, record)

	// Calculate the sustained duration
	return d.ShouldFix()
}

func (d *DeviationDetector[T]) ShouldFix() (bool, time.Duration) {
	if len(d.records) == 0 {
		return false, 0
	}

	last := d.records[len(d.records)-1]
	firstRecord := d.records[0]
	sustainedDuration := last.Time.Sub(firstRecord.Time)
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
