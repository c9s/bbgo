package grid2

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsGridNum                                *prometheus.GaugeVec
	metricsGridNumOfOrders                        *prometheus.GaugeVec
	metricsGridNumOfOrdersWithCorrectPrice        *prometheus.GaugeVec
	metricsGridNumOfMissingOrders                 *prometheus.GaugeVec
	metricsGridNumOfMissingOrdersWithCorrectPrice *prometheus.GaugeVec
	metricsGridProfit                             *prometheus.GaugeVec

	metricsGridUpperPrice      *prometheus.GaugeVec
	metricsGridLowerPrice      *prometheus.GaugeVec
	metricsGridQuoteInvestment *prometheus.GaugeVec
	metricsGridBaseInvestment  *prometheus.GaugeVec

	metricsGridFilledOrderPrice *prometheus.GaugeVec

	metricsNumOfOpenOrders *prometheus.GaugeVec
)

func labelKeys(labels prometheus.Labels) []string {
	var keys []string
	for k := range labels {
		keys = append(keys, k)
	}

	return keys
}

func mergeLabels(a, b prometheus.Labels) prometheus.Labels {
	labels := prometheus.Labels{}
	for k, v := range a {
		labels[k] = v
	}

	for k, v := range b {
		labels[k] = v
	}
	return labels
}

func initMetrics(extendedLabels []string) {
	if metricsGridNum != nil {
		return
	}

	metricsGridNum = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_num",
			Help: "number of grids",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

	metricsGridNumOfOrders = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_num_of_orders",
			Help: "number of orders",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

	metricsGridNumOfOrdersWithCorrectPrice = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_num_of_correct_price_orders",
			Help: "number of orders with correct grid prices",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

	metricsGridNumOfMissingOrders = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_num_of_missing_orders",
			Help: "number of missing orders",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

	metricsGridNumOfMissingOrdersWithCorrectPrice = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_num_of_missing_correct_price_orders",
			Help: "number of missing orders with correct prices",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

	metricsGridProfit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_profit",
			Help: "realized grid profit",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

	metricsGridUpperPrice = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_upper_price",
			Help: "the upper price of grid",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

	metricsGridLowerPrice = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_lower_price",
			Help: "the lower price of grid",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

	metricsGridQuoteInvestment = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_quote_investment",
			Help: "the quote investment of grid",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

	metricsGridBaseInvestment = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_base_investment",
			Help: "the base investment of grid",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

	metricsGridFilledOrderPrice = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_filled_order_price",
			Help: "the price of filled grid order",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
			"side",
		}, extendedLabels...),
	)

	metricsNumOfOpenOrders = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_num_of_open_orders",
			Help: "number of open orders",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)

}

var metricsRegistered = false

func registerMetrics() {
	if metricsRegistered {
		return
	}

	if metricsGridNum == nil {
		// default setup
		initMetrics(nil)
	}

	prometheus.MustRegister(
		metricsGridNum,
		metricsGridNumOfOrders,
		metricsGridNumOfOrdersWithCorrectPrice,
		metricsGridNumOfMissingOrders,
		metricsGridNumOfMissingOrdersWithCorrectPrice,
		metricsGridProfit,
		metricsGridLowerPrice,
		metricsGridUpperPrice,
		metricsGridQuoteInvestment,
		metricsGridBaseInvestment,
		metricsGridFilledOrderPrice,
		metricsNumOfOpenOrders,
	)
	metricsRegistered = true
}
