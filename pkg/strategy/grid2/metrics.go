package grid2

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsGridNum                *prometheus.GaugeVec
	metricsGridNumOfOrders        *prometheus.GaugeVec
	metricsGridNumOfMissingOrders *prometheus.GaugeVec
	metricsGridOrderPrices        *prometheus.GaugeVec
	metricsGridProfit             *prometheus.GaugeVec
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

	metricsGridOrderPrices = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_order_prices",
			Help: "order prices",
		},
		append([]string{
			"exchange", // exchange name
			"symbol",   // symbol of the market
			"ith",
			"side",
		}, extendedLabels...),
	)

	metricsGridProfit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_grid_profit",
			Help: "realized grid profit",
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
		metricsGridNumOfMissingOrders,
		metricsGridProfit,
		metricsGridOrderPrices,
	)
	metricsRegistered = true
}