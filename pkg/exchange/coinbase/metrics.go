package coinbase

import (
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
)

// Error type constants for metrics
type MetricOrderErrorType string

const (
	ErrorTypeUnknown             MetricOrderErrorType = "unknown"
	ErrorTypeRateLimit           MetricOrderErrorType = "rate_limit"
	ErrorTypeInsufficientBalance MetricOrderErrorType = "insufficient_balance"
	ErrorTypeInvalidRequest      MetricOrderErrorType = "invalid_request"
	ErrorTypeTimeout             MetricOrderErrorType = "timeout"
	ErrorTypeOrderNotFound       MetricOrderErrorType = "order_not_found"
	ErrorTypeAlreadyCanceled     MetricOrderErrorType = "already_canceled"
)

// Status constants for metrics
type MetricStatus string

const (
	StatusSuccess MetricStatus = "success"
	StatusError   MetricStatus = "error"
)

var (
	// Order Submission Metrics
	orderSubmissionLatencyMetrics = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "coinbase_order_submission_duration_milliseconds",
			Help:    "Order submission duration from request to response in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2.0, 15), // 1ms to ~32s
		}, []string{"symbol", "side", "type"},
	)

	orderSubmissionTotalMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coinbase_order_submission_total",
			Help: "Total number of order submissions",
		}, []string{"symbol", "side", "type", "status"},
	)

	orderSubmissionErrorMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coinbase_order_submission_errors_total",
			Help: "Total number of order submission errors",
		}, []string{"symbol", "side", "type", "error_type"},
	)

	// Order Cancel Metrics
	orderCancelLatencyMetrics = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "coinbase_order_cancel_latency_milliseconds",
			Help:    "Time from cancel request to cancel confirmation",
			Buckets: prometheus.ExponentialBuckets(1, 2.0, 12), // 1ms to ~4s
		}, []string{"symbol"},
	)

	orderCancelTotalMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coinbase_order_cancel_total",
			Help: "Total number of order cancellation attempts",
		}, []string{"symbol", "side", "type", "status"},
	)

	orderCancelErrorMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coinbase_order_cancel_errors_total",
			Help: "Total number of order cancellation errors",
		}, []string{"symbol", "side", "type", "error_type"},
	)
)

func init() {
	prometheus.MustRegister(
		orderSubmissionLatencyMetrics,
		orderSubmissionTotalMetrics,
		orderSubmissionErrorMetrics,
		orderCancelLatencyMetrics,
		orderCancelTotalMetrics,
		orderCancelErrorMetrics,
	)
}

// Helper function to record order submission metrics
func recordOrderSubmissionMetrics(order types.SubmitOrder, duration time.Duration, err error) {
	symbol := string(order.Symbol)

	// Record submission duration in milliseconds
	orderSubmissionLatencyMetrics.With(prometheus.Labels{
		"symbol": symbol,
		"side":   string(order.Side),
		"type":   string(order.Type),
	}).Observe(float64(duration.Nanoseconds()) / 1e6)

	// Record submission status
	status := StatusSuccess
	if err != nil {
		status = StatusError
		// Categorize error type
		errorType := ErrorTypeUnknown
		if strings.Contains(err.Error(), "rate limit") {
			errorType = ErrorTypeRateLimit
		} else if strings.Contains(err.Error(), "insufficient") {
			errorType = ErrorTypeInsufficientBalance
		} else if strings.Contains(err.Error(), "invalid") {
			errorType = ErrorTypeInvalidRequest
		} else if strings.Contains(err.Error(), "timeout") {
			errorType = ErrorTypeTimeout
		}

		orderSubmissionErrorMetrics.With(prometheus.Labels{
			"symbol":     symbol,
			"side":       string(order.Side),
			"type":       string(order.Type),
			"error_type": string(errorType),
		}).Inc()
	}

	orderSubmissionTotalMetrics.With(prometheus.Labels{
		"symbol": symbol,
		"side":   string(order.Side),
		"type":   string(order.Type),
		"status": string(status),
	}).Inc()
}

// Helper function to record order cancellation metrics
func recordOrderCancelMetrics(order types.Order, duration time.Duration, err error) {
	symbol := string(order.Symbol)

	// Record cancellation latency
	orderCancelLatencyMetrics.With(prometheus.Labels{
		"symbol": symbol,
	}).Observe(float64(duration.Nanoseconds()) / 1e6)

	// Record cancellation result in total metrics
	status := StatusSuccess
	if err != nil {
		status = StatusError
		// Categorize cancellation error types
		errorType := ErrorTypeUnknown
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			errorType = ErrorTypeOrderNotFound
		} else if strings.Contains(err.Error(), "already canceled") {
			errorType = ErrorTypeAlreadyCanceled
		} else if strings.Contains(err.Error(), "rate limit") {
			errorType = ErrorTypeRateLimit
		} else if strings.Contains(err.Error(), "timeout") {
			errorType = ErrorTypeTimeout
		}

		// Record the specific cancellation error
		orderCancelErrorMetrics.With(prometheus.Labels{
			"symbol":     symbol,
			"side":       string(order.Side),
			"type":       string(order.Type),
			"error_type": string(errorType),
		}).Inc()
	}

	// Record cancellation attempt total
	orderCancelTotalMetrics.With(prometheus.Labels{
		"symbol": symbol,
		"side":   string(order.Side),
		"type":   string(order.Type),
		"status": string(status),
	}).Inc()
}
