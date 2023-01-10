package grid2

import "github.com/prometheus/client_golang/prometheus"

var (
	metricsGridNumOfOrders *prometheus.GaugeVec
	metricsGridOrderPrices *prometheus.GaugeVec
	metricsGridProfit      *prometheus.GaugeVec
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
	metricsGridNumOfOrders = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_num_of_orders",
			Help: "number of orders",
		},
		append([]string{
			"strategy_instance",
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
			"strategy_instance",
			"exchange", // exchange name
			"symbol",   // symbol of the market
			"ith",
		}, extendedLabels...),
	)

	metricsGridProfit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_grid2_grid_profit",
			Help: "realized grid profit",
		},
		append([]string{
			"strategy_instance",
			"exchange", // exchange name
			"symbol",   // symbol of the market
		}, extendedLabels...),
	)
}

func registerMetrics() {
	prometheus.MustRegister(
		metricsGridNumOfOrders,
		metricsGridProfit,
		metricsGridOrderPrices,
	)
}
