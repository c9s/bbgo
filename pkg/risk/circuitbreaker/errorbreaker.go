package circuitbreaker

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

var errorCntMetric = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_error_breaker_consecutive_error_count",
		Help: "Current count of consecutive errors tracked by the error breaker",
	},
	[]string{"strategy", "strategyInstance"},
)
var errorHaltMetric = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_error_breaker_halt",
		Help: "Indicates whether the error breaker is currently in a halted state (1 for halted, 0 for active)",
	},
	[]string{"strategy", "strategyInstance"},
)

// ErrorRecord stores an error along with its timestamp
type ErrorRecord struct {
	timestamp time.Time
	err       error
}

// ErrorBreaker is a circuit breaker that tracks consecutive errors
// and halts operations if the error count exceeds the threshold.
//
//go:generate callbackgen -type ErrorBreaker
type ErrorBreaker struct {
	mu sync.RWMutex

	// breaker configuration
	Enabled                  bool           `json:"enabled"`
	MaxConsecutiveErrorCount int            `json:"maxConsecutiveErrorCount"`
	HaltDuration             types.Duration `json:"haltDuration"`

	// breaker state
	errors   []ErrorRecord
	halted   bool
	haltedAt time.Time

	// haltCallbacks are the callbacks that will be called when the breaker is halted.
	// The callbacks will be called when the breaker is locked.
	// As a result, the callbacks should not call any methods that require locking the breaker again.
	// Ideally, the callbacks should just make use of the passed parameters to perform their actions.
	haltCallbacks []func(haltedAt time.Time, records []ErrorRecord)

	// error breaker metrics
	strategyInstance string
	errorCntMetric   prometheus.Gauge
	errorHaltMetric  prometheus.Gauge
}

// NewErrorBreaker creates a new ErrorBreaker with the given parameters.
// maxErrors: maximum number of consecutive errors allowed
// haltDuration: duration for which the breaker will be halted
func NewErrorBreaker(strategy, strategyInstance string, maxErrors int, haltDuration types.Duration) *ErrorBreaker {
	if maxErrors <= 0 {
		log.Warnf("the maxErrors cannot be negative, fallback to 5: %d", maxErrors)
		maxErrors = 5
	}
	b := &ErrorBreaker{
		Enabled:                  true,
		MaxConsecutiveErrorCount: maxErrors,
		HaltDuration:             haltDuration,
		errors:                   make([]ErrorRecord, 0, maxErrors),
	}
	b.SetMetricsInfo(strategy, strategyInstance)
	b.updateMetrics()
	return b
}

func (b *ErrorBreaker) SetMetricsInfo(strategy, strategyInstance string) {
	labels := prometheus.Labels{"strategy": strategy, "strategyInstance": strategyInstance}
	b.strategyInstance = strategyInstance
	b.errorCntMetric = errorCntMetric.With(labels)
	b.errorHaltMetric = errorHaltMetric.With(labels)
	b.updateMetrics()
}

// RecordError records a critical error and updates the circuit breaker state.
// If err is nil, the breaker is reset.
// err: the error that occurred (if nil, the breaker is reset)
func (b *ErrorBreaker) RecordError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	defer b.updateMetrics()

	b.recordError(time.Now(), err)
}

func (b *ErrorBreaker) recordError(now time.Time, err error) {
	// If no error occurred, reset the breaker
	if err == nil {
		b.reset()
		return
	}

	// Add the new error record
	b.errors = append(b.errors, ErrorRecord{
		timestamp: now,
		err:       err,
	})

	// the breaker is already halted
	// keep halted until the duration expires
	if b.halted {
		if len(b.errors) > b.MaxConsecutiveErrorCount {
			// drop the oldest error record to prevent unbounded growth
			b.errors = b.errors[1:]
		}
		return
	}

	// the breaker is not halted yet
	// check if we've exceeded the max errors threshold
	if len(b.errors) >= b.MaxConsecutiveErrorCount {
		// trigger halt
		b.EmitHalt(now, b.errors)
		b.halted = true
		b.haltedAt = now
	}
}

// IsHalted returns whether the circuit breaker is in a halted state.
// If the breaker is halted and the halt duration has expired, it automatically resets the breaker.
func (b *ErrorBreaker) IsHalted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.Enabled {
		return false
	}

	isHalted := b.isHalted(time.Now())
	b.updateMetrics()

	return isHalted
}

func (b *ErrorBreaker) isHalted(now time.Time) bool {
	// If not halted, return false immediately
	if !b.halted {
		return false
	}

	// Check if the halt duration has expired
	if !b.haltedAt.IsZero() && now.Sub(b.haltedAt) >= b.HaltDuration.Duration() {
		// Halt duration has expired, reset the breaker
		b.reset()
	}

	return b.halted
}

// Reset resets the circuit breaker, clearing all recorded errors and the halted state.
func (b *ErrorBreaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.reset()
}

func (b *ErrorBreaker) reset() {
	if b.errors != nil {
		b.errors = b.errors[:0]
	} else {
		b.errors = make([]ErrorRecord, 0, b.MaxConsecutiveErrorCount)
	}
	b.halted = false
	b.haltedAt = time.Time{}
	b.updateMetrics()
}

// ErrorCount returns the current number of errors tracked.
func (b *ErrorBreaker) ErrorCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.errors)
}

// Errors returns a copy of all errors currently tracked.
func (b *ErrorBreaker) Errors() []error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []error
	for _, record := range b.errors {
		result = append(result, record.err)
	}

	return result
}

func (b *ErrorBreaker) SlackAttachment() slack.Attachment {
	b.mu.RLock()
	defer b.mu.RUnlock()

	errorCount := len(b.errors)

	// Build error details text
	var errorDetails strings.Builder
	if errorCount > 0 {
		errorDetails.WriteString(fmt.Sprintf("Errors encountered (%s):\n", b.strategyInstance))
		for i, record := range b.errors {
			errorDetails.WriteString(fmt.Sprintf("%d. [%s] %v\n",
				i+1,
				record.timestamp.Format(time.RFC3339),
				record.err,
			))
		}
	}

	status := "ACTIVE"
	title := "✅ Error Circuit Breaker ACTIVE"
	color := "#228B22"
	if b.halted {
		status = "HALTED"
		title = "🛑 Error Circuit Breaker HALTED"
		color = "danger"
	}

	fields := []slack.AttachmentField{
		{Title: "Status", Value: status, Short: true},
		{Title: "Error Count", Value: fmt.Sprintf("%d / %d", errorCount, b.MaxConsecutiveErrorCount), Short: true},
	}

	if len(b.errors) > 0 {
		lastError := b.errors[0]

		for _, record := range b.errors {
			if record.timestamp.After(lastError.timestamp) {
				lastError = record
			}
		}

		fields = append(fields,
			slack.AttachmentField{
				Title: "Last Error At",
				Value: lastError.timestamp.Format(time.RFC3339),
				Short: true,
			},
		)
		if !b.haltedAt.IsZero() {
			fields = append(fields,
				slack.AttachmentField{
					Title: "Halted At",
					Value: b.haltedAt.Format(time.RFC3339),
					Short: true,
				},
			)
		}
	}

	if !b.haltedAt.IsZero() {
		recoveryTime := b.haltedAt.Add(b.HaltDuration.Duration())
		fields = append(fields,
			slack.AttachmentField{
				Title: "Halt Duration",
				Value: b.HaltDuration.Duration().String(),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Recovery At",
				Value: recoveryTime.Format(time.RFC3339),
				Short: true,
			},
		)
	}

	return slack.Attachment{
		Color:  color,
		Title:  title,
		Text:   errorDetails.String(),
		Fields: fields,
	}
}

func (b *ErrorBreaker) updateMetrics() {
	b.errorCntMetric.Set(float64(len(b.errors)))
	if b.halted {
		b.errorHaltMetric.Set(1)
	} else {
		b.errorHaltMetric.Set(0)
	}
}
