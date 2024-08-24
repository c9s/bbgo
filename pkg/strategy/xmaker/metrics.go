package xmaker

import "github.com/prometheus/client_golang/prometheus"

var openOrderExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_open_order_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol", "side"})

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

var numOfLayersMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xmaker_num_of_layers",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

func init() {
	prometheus.MustRegister(
		openOrderExposureInUsdMetrics,
		makerBestBidPriceMetrics,
		makerBestAskPriceMetrics,
		numOfLayersMetrics,
	)
}
