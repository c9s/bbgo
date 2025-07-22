package coinbase

import (
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Order Submission Metrics
	orderSubmissionLatencyMetrics = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "coinbase_order_submission_duration_milliseconds",
			Help:    "Order submission duration from request to response in milliseconds (successful requests only)",
			Buckets: prometheus.LinearBuckets(50, 25, 19), // 50ms to ~500ms
		}, []string{"symbol", "side", "type"},
	)

	orderSubmissionTotalMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coinbase_order_submission_total",
			Help: "Total number of order submissions",
		}, []string{"symbol", "side", "type", "success"},
	)

	orderSubmissionErrorCodeMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coinbase_order_submission_error_codes_total",
			Help: "Total number of order submission errors by error type",
		}, []string{"symbol", "side", "type", "status_code"},
	)

	// Order Cancel Metrics
	orderCancelLatencyMetrics = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "coinbase_order_cancel_latency_milliseconds",
			Help:    "Time from cancel request to cancel confirmation (successful requests only)",
			Buckets: prometheus.LinearBuckets(50, 25, 19), // 50ms to ~500ms
		}, []string{"symbol"},
	)

	orderCancelTotalMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coinbase_order_cancel_total",
			Help: "Total number of order cancellation attempts",
		}, []string{"symbol", "side", "type", "success"},
	)

	orderCancelErrorCodeMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coinbase_order_cancel_error_codes_total",
			Help: "Total number of order cancellation errors by error type",
		}, []string{"symbol", "side", "type", "status_code"},
	)
)

func init() {
	prometheus.MustRegister(
		orderSubmissionLatencyMetrics,
		orderSubmissionTotalMetrics,
		orderSubmissionErrorCodeMetrics,
		orderCancelLatencyMetrics,
		orderCancelTotalMetrics,
		orderCancelErrorCodeMetrics,
	)
}

// Helper function to record successful order submission metrics
func recordSuccessOrderSubmissionMetrics(order types.SubmitOrder, duration time.Duration) {
	symbol := string(order.Symbol)

	orderSubmissionLatencyMetrics.With(prometheus.Labels{
		"symbol": symbol,
		"side":   string(order.Side),
		"type":   string(order.Type),
	}).Observe(float64(duration.Milliseconds()))

	orderSubmissionTotalMetrics.With(prometheus.Labels{
		"symbol":  symbol,
		"side":    string(order.Side),
		"type":    string(order.Type),
		"success": "true",
	}).Inc()
}

// Helper function to record failed order submission metrics
func recordFailedOrderSubmissionMetrics(order types.SubmitOrder, err *requestgen.ErrResponse) {
	symbol := string(order.Symbol)

	orderSubmissionErrorCodeMetrics.With(prometheus.Labels{
		"symbol":      symbol,
		"side":        string(order.Side),
		"type":        string(order.Type),
		"status_code": strconv.Itoa(err.StatusCode),
	}).Inc()
	orderSubmissionTotalMetrics.With(prometheus.Labels{
		"symbol":  symbol,
		"side":    string(order.Side),
		"type":    string(order.Type),
		"success": "false",
	}).Inc()
}

// Helper function to record order cancellation metrics
func recordSuccessOrderCancelMetrics(order types.Order, duration time.Duration) {
	symbol := string(order.Symbol)

	orderCancelLatencyMetrics.With(prometheus.Labels{
		"symbol": symbol,
	}).Observe(float64(duration.Milliseconds()))

	orderCancelTotalMetrics.With(prometheus.Labels{
		"symbol":  symbol,
		"side":    string(order.Side),
		"type":    string(order.Type),
		"success": "true",
	}).Inc()
}

func recordFailedOrderCancelMetrics(order types.Order, err *requestgen.ErrResponse) {
	symbol := string(order.Symbol)

	orderCancelErrorCodeMetrics.With(prometheus.Labels{
		"symbol":      symbol,
		"side":        string(order.Side),
		"type":        string(order.Type),
		"status_code": strconv.Itoa(err.StatusCode),
	}).Inc()
	orderCancelTotalMetrics.With(prometheus.Labels{
		"symbol":  symbol,
		"side":    string(order.Side),
		"type":    string(order.Type),
		"success": "false",
	}).Inc()
}
