package xfundingv2

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// TODO: integrate `dynamic.InitializeConfigMetrics` for automatic metric labels generation based on the config struct fields.
// see xmaker for example usage

var fundingRateMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xfundingv2_funding_rate",
		Help: "Funding rate of the symbol",
	},
	[]string{"symbol"},
)

var annualizedFundingRateMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xfundingv2_annualized_funding_rate",
		Help: "Annualized funding rate of the symbol",
	},
	[]string{"symbol"},
)

var roundTotalPnLMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xfundingv2_round_total_pnl",
		Help: "Total PnL of the arbitrage round",
	},
	[]string{"strategy_id", "symbol"},
)

var roundPositionFilledRatioMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xfundingv2_round_position_filled_ratio",
		Help: "Filled ratio of the position in the arbitrage round. It should be increasing up to 1 when round is opening and decreasing down to 0 when round is closing",
	},
	[]string{"strategy_id", "symbol", "accountType"},
)

var roundPositionMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xfundingv2_round_position",
		Help: "position of the arbitrage round",
	},
	[]string{"strategy_id", "symbol", "accountType"},
)

var roundQuantityDeviationMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xfundingv2_round_quantity_deviation",
		Help: "Quantity deviation of the arbitrage round",
	},
	[]string{"strategy_id", "symbol"},
)

var tickDurationMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xfundingv2_tick_duration",
		Help: "Duration of the tick in seconds",
	},
	[]string{"strategy_id"},
)
