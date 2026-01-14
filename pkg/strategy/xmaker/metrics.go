package xmaker

import "github.com/prometheus/client_golang/prometheus"

var delayedHedgeCounterMetrics = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "xmaker_delayed_hedge_counter",
		Help: "delay hedge counter",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var delayedHedgeMaxDurationMetrics = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "xmaker_delayed_hedge_max_duration",
		Help:    "delay hedge max duration in milliseconds",
		Buckets: prometheus.ExponentialBuckets(50, 2.0, 13),
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var cancelOrderDurationMetrics = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "xmaker_cancel_order_duration_milliseconds",
		Help:    "cancel order duration in milliseconds",
		Buckets: prometheus.ExponentialBuckets(50, 2.0, 13),
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var makerOrderPlacementDurationMetrics = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "xmaker_maker_order_placement_duration_milliseconds",
		Help:    "maker order placement duration in milliseconds",
		Buckets: prometheus.ExponentialBuckets(50, 2.0, 13),
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var openOrderBidExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_open_order_bid_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var openOrderAskExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_open_order_ask_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var makerBestBidPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_maker_best_bid_price",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var makerBestAskPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_maker_best_ask_price",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var bidMarginMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_bid_margin",
		Help: "the current bid margin (dynamic)",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var askMarginMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_ask_margin",
		Help: "the current ask margin (dynamic)",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var aggregatedSignalMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_aggregated_signal",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var netProfitMarginHistogram = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "xmaker_net_profit_margin",
		Help:    "net profit",
		Buckets: prometheus.ExponentialBuckets(0.001, 2.0, 10),
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var spreadMakerCounterMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_spread_maker_counter",
		Help: "spread maker counter",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var spreadMakerVolumeMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_spread_maker_volume",
		Help: "spread maker volume",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var spreadMakerQuoteVolumeMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_spread_maker_quote_volume",
		Help: "spread maker quote volume",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var coveredPositionMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_covered_position",
		Help: "covered position",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

// Divergence (D2) and components
var divergenceD2 = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_divergence_d2",
		Help: "Direction divergence score D2 (1 - p_sign) for xmaker",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var directionMean = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_direction_mean",
		Help: "Weighted mean of normalized signals (m)",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var directionAlignedWeight = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_direction_aligned_weight",
		Help: "Aligned weight proportion p_sign",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var splitHedgeBidHigherThanAskCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "xmaker_split_hedge_bid_higher_than_ask_total",
		Help: "The total number of times the split hedge bid price was higher than the ask price",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var splitHedgeWeightedBidPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_split_hedge_weighted_bid_price",
		Help: "The weighted bid price calculated by split hedge",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var splitHedgeWeightedAskPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_split_hedge_weighted_ask_price",
		Help: "The weighted ask price calculated by split hedge",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

func init() {
	prometheus.MustRegister(
		openOrderBidExposureInUsdMetrics,
		openOrderAskExposureInUsdMetrics,
		makerBestBidPriceMetrics,
		makerBestAskPriceMetrics,
		bidMarginMetrics,
		askMarginMetrics,
		aggregatedSignalMetrics,
		cancelOrderDurationMetrics,
		makerOrderPlacementDurationMetrics,
		delayedHedgeCounterMetrics,
		delayedHedgeMaxDurationMetrics,
		netProfitMarginHistogram,

		spreadMakerCounterMetrics,
		spreadMakerVolumeMetrics,
		spreadMakerQuoteVolumeMetrics,

		coveredPositionMetrics,
		divergenceD2,
		directionMean,
		directionAlignedWeight,
		splitHedgeBidHigherThanAskCounter,
		splitHedgeWeightedBidPriceMetrics,
		splitHedgeWeightedAskPriceMetrics,
	)
}
