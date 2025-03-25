package xdepthmaker

import "github.com/prometheus/client_golang/prometheus"

var pendingOrderTiersCountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_pending_order_tiers_count",
		Help: "the number of pending order tiers",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "side", "percentage", "symbol"},
)
