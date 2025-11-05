package xpremium

import "github.com/prometheus/client_golang/prometheus"

var (
	premiumRatioHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "xpremium_premium_ratio",
			Help:    "premium ratio = (premiumBid - baseAsk)/baseAsk per comparison",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2.0, 12), // 0.01% .. ~20%
		}, []string{"strategy_type", "strategy_id", "base_session", "premium_session", "symbol"},
	)

	discountRatioHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "xpremium_discount_ratio",
			Help:    "discount ratio = (premiumAsk - baseBid)/premiumAsk per comparison (abs value)",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2.0, 12),
		}, []string{"strategy_type", "strategy_id", "base_session", "premium_session", "symbol"},
	)

	signalCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "xpremium_signal_total",
			Help: "number of LONG/SHORT signals emitted when spread threshold is hit",
		}, []string{"strategy_type", "strategy_id", "base_session", "premium_session", "symbol", "side"},
	)

	signalRatioHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "xpremium_signal_ratio",
			Help:    "observed spread ratio at the time a LONG/SHORT signal is triggered (abs value)",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2.0, 12),
		}, []string{"strategy_type", "strategy_id", "base_session", "premium_session", "symbol", "side"},
	)
)

func init() {
	prometheus.MustRegister(
		premiumRatioHistogram,
		discountRatioHistogram,
		signalCounter,
		signalRatioHistogram,
	)
}
