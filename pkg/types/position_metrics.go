package types

import "github.com/prometheus/client_golang/prometheus"

var positionAverageCostMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_avg_cost",
		Help: "bbgo position average cost metrics",
	}, []string{"strategy_id", "strategy_type", "symbol"})

var positionBaseQuantityMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_base_qty",
		Help: "bbgo position base quantity metrics",
	}, []string{"strategy_id", "strategy_type", "symbol"})

var positionQuoteQuantityMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_quote_qty",
		Help: "bbgo position quote quantity metrics",
	}, []string{"strategy_id", "strategy_type", "symbol"})

func init() {
	prometheus.MustRegister(
		positionAverageCostMetrics,
		positionBaseQuantityMetrics,
		positionQuoteQuantityMetrics,
	)
}
