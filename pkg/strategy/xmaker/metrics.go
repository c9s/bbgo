package xmaker

import "github.com/prometheus/client_golang/prometheus"

var delayedHedgeCounterMetrics = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "xmaker_delayed_hedge_counter",
		Help: "delay hedge counter",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var delayedHedgeMaxDurationMetrics = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "xmaker_delayed_hedge_max_duration",
		Help:    "delay hedge max duration in milliseconds",
		Buckets: prometheus.ExponentialBuckets(50, 2.0, 13),
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var cancelOrderDurationMetrics = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "xmaker_cancel_order_duration_milliseconds",
		Help:    "cancel order duration in milliseconds",
		Buckets: prometheus.ExponentialBuckets(50, 2.0, 13),
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var makerOrderPlacementDurationMetrics = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "xmaker_maker_order_placement_duration_milliseconds",
		Help:    "maker order placement duration in milliseconds",
		Buckets: prometheus.ExponentialBuckets(50, 2.0, 13),
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var openOrderBidExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_open_order_bid_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var openOrderAskExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_open_order_ask_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var makerBestBidPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_maker_best_bid_price",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var makerBestAskPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_maker_best_ask_price",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var bidMarginMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_bid_margin",
		Help: "the current bid margin (dynamic)",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var askMarginMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_ask_margin",
		Help: "the current ask margin (dynamic)",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var aggregatedSignalMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_aggregated_signal",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var netProfitMarginHistogram = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "xmaker_net_profit_margin",
		Help:    "net profit",
		Buckets: prometheus.ExponentialBuckets(0.001, 2.0, 10),
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var spreadMakerCounterMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_spread_maker_counter",
		Help: "spread maker counter",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var spreadMakerVolumeMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_spread_maker_volume",
		Help: "spread maker volume",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var spreadMakerQuoteVolumeMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_spread_maker_quote_volume",
		Help: "spread maker quote volume",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

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
	)
}
