package xdepthmaker

import "github.com/prometheus/client_golang/prometheus"

var openOrdersCountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_open_order_count",
		Help: "the number of pending order tiers",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "side", "price_range", "symbol"},
)

var openOrderExposureInUsdtMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_open_order_exposure_in_usdt",
		Help: "the total exposure in USDT",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "side", "price_range", "symbol"},
)

func init() {
	prometheus.MustRegister(
		openOrdersCountMetrics,
		openOrderExposureInUsdtMetrics,
	)
}
