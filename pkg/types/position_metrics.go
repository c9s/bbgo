package types

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var positionAverageCostMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_avg_cost",
		Help: "bbgo position average cost metrics",
	}, []string{"strategy_id", "strategy_type", "symbol"})

var positionBaseQuantityMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_base_qty",
		Help: "bbgo position base quantity metrics",
	}, []string{"strategy_id", "strategy_type", "symbol"})

var positionQuoteQuantityMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_quote_qty",
		Help: "bbgo position quote quantity metrics",
	}, []string{"strategy_id", "strategy_type", "symbol"})

var positionUnrealizedProfitMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_unrealized_profit",
		Help: "bbgo position unrealized profit metrics",
	}, []string{"strategy_id", "strategy_type", "symbol"})

var positionRealizedProfitMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_realized_profit",
		Help: "bbgo position cumulative realized profit (gross, before fees)",
	}, []string{"strategy_id", "strategy_type", "symbol"})

var positionRealizedNetProfitMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_realized_net_profit",
		Help: "bbgo position cumulative realized net profit (after fees)",
	}, []string{"strategy_id", "strategy_type", "symbol"})
