package xmaker

import "github.com/prometheus/client_golang/prometheus"

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

var configNumOfLayersMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_config_num_of_layers",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "symbol"})

var configMaxExposureMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_config_max_exposure",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "symbol"})

var configBidMarginMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_config_bid_margin",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "symbol"})

var configAskMarginMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_config_ask_margin",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "symbol"})

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
		configNumOfLayersMetrics,
		configMaxExposureMetrics,
		configBidMarginMetrics,
		configAskMarginMetrics,
	)
}
