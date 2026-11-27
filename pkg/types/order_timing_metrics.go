package types

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// The latency kinds and the drop reasons of orderTimingRejectedMetrics.
const (
	ackLatencyKind    = "ack"
	cancelLatencyKind = "cancel"

	reasonNegativeRaw      = "negative_raw"
	reasonNegativeAdjusted = "negative_adjusted"
	reasonNoClockOffset    = "no_clock_offset"
)

// The outcomes of orderCancelOutcomeMetrics.
const (
	cancelOutcomeCancelled = "cancelled"
	cancelOutcomeRaced     = "raced"
)

// The metrics below are reported by OrderTimingTracker, from the order lifecycle
// timestamps it already collects.
//
// They run from a local send timestamp to the timestamp the exchange itself put on the
// order, which spans two clocks, so each latency is reported twice: raw and corrected by
// the estimated clock offset (via ClockOffsetEstimator). Compare the two series before
// trusting the correction; the raw series is the one to keep if the estimate turns out to
// be unreliable.

var orderAckLatencyMetrics = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "bbgo_order_ack_latency_milliseconds",
		Help:    "latency from sending an order to the exchange order creation timestamp in milliseconds, not corrected for the clock offset",
		Buckets: prometheus.ExponentialBuckets(5, 2.0, 11),
	}, []string{"symbol"})

var orderAckLatencyAdjustedMetrics = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "bbgo_order_ack_latency_adjusted_milliseconds",
		Help:    "latency from sending an order to the exchange order creation timestamp in milliseconds, corrected for the estimated clock offset",
		Buckets: prometheus.ExponentialBuckets(5, 2.0, 11),
	}, []string{"symbol"})

var orderCancelAckLatencyMetrics = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "bbgo_order_cancel_ack_latency_milliseconds",
		Help:    "latency from sending a cancel request to the exchange cancellation timestamp in milliseconds, not corrected for the clock offset",
		Buckets: prometheus.ExponentialBuckets(5, 2.0, 11),
	}, []string{"symbol"})

var orderCancelAckLatencyAdjustedMetrics = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "bbgo_order_cancel_ack_latency_adjusted_milliseconds",
		Help:    "latency from sending a cancel request to the exchange cancellation timestamp in milliseconds, corrected for the estimated clock offset",
		Buckets: prometheus.ExponentialBuckets(5, 2.0, 11),
	}, []string{"symbol"})

// orderCancelOutcomeMetrics counts how each cancel request finished: "cancelled" when the
// order came back cancelled with no further fill, "raced" when the order filled further
// between sending the cancel and the exchange acting on it.
var orderCancelOutcomeMetrics = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "bbgo_order_cancel_outcome_total",
		Help: "number of finished cancel requests by outcome",
	}, []string{"symbol", "outcome"})

// orderCancelRaceFilledVolumeMetrics counts the volume filled between sending a cancel
// request and the exchange acting on it, which is the volume the cancel failed to pull.
var orderCancelRaceFilledVolumeMetrics = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "bbgo_order_cancel_race_filled_volume_total",
		Help: "volume filled after a cancel request was sent but before the exchange cancelled the order",
	}, []string{"symbol"})

// orderTimingRecordsMetrics tracks how many orders OrderTimingTracker currently holds, to
// detect records that are never pruned.
var orderTimingRecordsMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_order_timing_records",
		Help: "number of orders currently held by the order timing tracker",
	}, []string{"symbol"})

// orderTimingRejectedMetrics counts the latency observations that were dropped instead of
// reported, by latency kind ("ack", "cancel") and reason. A persistently non-zero
// "negative_raw" means the clock offset is large enough to make the uncorrected latency
// meaningless, "negative_adjusted" means the offset estimate is overshooting, and
// "no_clock_offset" means no estimate was available yet.
var orderTimingRejectedMetrics = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "bbgo_order_timing_rejected_total",
		Help: "number of order timing latency observations dropped instead of reported, by kind and reason",
	}, []string{"symbol", "kind", "reason"})

// orderTimingClockOffsetMetrics reports the estimated exchange clock minus the local clock
// in milliseconds, per symbol. Watch it for drift: a step change means one of the two
// clocks moved, and the adjusted latency series around that step should not be compared.
var orderTimingClockOffsetMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_order_timing_clock_offset_milliseconds",
		Help: "estimated exchange clock minus the local clock in milliseconds",
	}, []string{"symbol"})

// hasExchangeTimestamp reports whether t is a timestamp the exchange really sent. A missing
// millisecond timestamp converts to the unix epoch rather than the zero time, so IsZero alone
// does not catch it.
func hasExchangeTimestamp(t time.Time) bool {
	return !t.IsZero() && t.Unix() > 0
}

// reporting reports whether the tracker has a symbol to label its metrics with.
func (s *OrderTimingTracker) reporting() bool {
	return s != nil && s.symbol != ""
}

func (s *OrderTimingTracker) symbolLabels() prometheus.Labels {
	return prometheus.Labels{"symbol": s.symbol}
}

// observeClockSample feeds one exchange timestamp and the local time it arrived to the clock
// offset estimator. The estimator rejects a missing or implausible timestamp itself.
func (s *OrderTimingTracker) observeClockSample(serverTime, localTime time.Time) {
	if s == nil || s.clock == nil {
		return
	}

	s.clock.Observe(serverTime, localTime)
}

// clockOffset returns the estimated exchangeClock - localClock, publishing it as a gauge so
// the correction applied to the adjusted latencies can be watched for drift.
func (s *OrderTimingTracker) clockOffset() (time.Duration, bool) {
	if s == nil || s.clock == nil {
		return 0, false
	}

	offset, ok := s.clock.Offset()
	if !ok {
		return 0, false
	}

	if s.reporting() {
		orderTimingClockOffsetMetrics.With(s.symbolLabels()).Set(float64(offset.Milliseconds()))
	}
	return offset, true
}

// reportAckLatency reports the latency from sending the order to the creation timestamp the
// exchange assigned it, once per order. Must be called with s.mu held.
func (s *OrderTimingTracker) reportAckLatency(record *OrderTimingRecord) {
	if !s.reporting() || record == nil || record.ackReported {
		return
	}
	// SendOrderAt is read from the local clock, so the zero time is the only missing value it
	// can hold. ServerCreatedAt comes from the exchange, where a missing millisecond timestamp
	// arrives as the unix epoch instead.
	if record.SendOrderAt.IsZero() || !hasExchangeTimestamp(record.ServerCreatedAt) {
		return
	}

	record.ackReported = true
	s.reportLatency(
		ackLatencyKind,
		orderAckLatencyMetrics,
		orderAckLatencyAdjustedMetrics,
		record.LatencyToServer(),
	)
}

// reportCancelOutcome reports the cancel latency and how the cancel finished, once per
// order. Must be called with s.mu held.
func (s *OrderTimingTracker) reportCancelOutcome(record *OrderTimingRecord) {
	if !s.reporting() || record == nil || record.cancelReported {
		return
	}
	// See reportAckLatency on why the two ends are guarded differently.
	if record.SendCancelAt.IsZero() || !hasExchangeTimestamp(record.ServerCancelledAt) {
		return
	}

	record.cancelReported = true
	s.reportLatency(
		cancelLatencyKind,
		orderCancelAckLatencyMetrics,
		orderCancelAckLatencyAdjustedMetrics,
		record.LatencyToCancel(),
	)

	// CancelWorked needs the snapshot of the order the exchange sent back. Without it the
	// outcome is unknown rather than raced, so only the latency is reported.
	if record.OrderAfterCancel == nil {
		return
	}

	// A cancel that pulled the order with no further fill is the outcome we want; the rest
	// lost the race against a taker, wholly or in part.
	outcome := cancelOutcomeRaced
	if record.CancelWorked() {
		outcome = cancelOutcomeCancelled
	}
	orderCancelOutcomeMetrics.WithLabelValues(s.symbol, outcome).Inc()

	if additional := record.AdditionalFilledVolume(); additional.Sign() > 0 {
		orderCancelRaceFilledVolumeMetrics.With(s.symbolLabels()).Add(additional.Float64())
	}
}

// reportLatency reports one latency that runs from a local timestamp to an exchange
// timestamp, both as measured and corrected by the estimated clock offset.
//
// A latency measured across two clocks can come out negative, which a histogram would
// silently record in its lowest bucket and so read as "very fast". Those observations are
// counted by reason instead of reported.
func (s *OrderTimingTracker) reportLatency(
	kind string,
	raw, adjusted *prometheus.HistogramVec,
	latency time.Duration,
) {
	labels := s.symbolLabels()

	if latency < 0 {
		orderTimingRejectedMetrics.WithLabelValues(s.symbol, kind, reasonNegativeRaw).Inc()
	} else {
		raw.With(labels).Observe(float64(latency.Milliseconds()))
	}

	offset, ok := s.clockOffset()
	if !ok {
		if s.clock != nil {
			orderTimingRejectedMetrics.WithLabelValues(s.symbol, kind, reasonNoClockOffset).Inc()
		}
		return
	}

	// The exchange end of the latency reads offset higher than the local end would, so the
	// offset comes back out of the measured duration.
	if corrected := latency - offset; corrected < 0 {
		orderTimingRejectedMetrics.WithLabelValues(s.symbol, kind, reasonNegativeAdjusted).Inc()
	} else {
		adjusted.With(labels).Observe(float64(corrected.Milliseconds()))
	}
}

// reportSize publishes how many orders the tracker holds. Must be called with s.mu held.
func (s *OrderTimingTracker) reportSize() {
	if !s.reporting() {
		return
	}

	orderTimingRecordsMetrics.With(s.symbolLabels()).Set(float64(len(s.orders)))
}
