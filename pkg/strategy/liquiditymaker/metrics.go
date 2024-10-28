package liquiditymaker

import "github.com/prometheus/client_golang/prometheus"

var openOrderBidExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_open_order_bid_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var openOrderAskExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_open_order_ask_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

func init() {
	prometheus.MustRegister(
		openOrderBidExposureInUsdMetrics,
		openOrderAskExposureInUsdMetrics,
	)
}
