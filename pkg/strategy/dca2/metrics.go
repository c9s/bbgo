package dca2

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsState             *prometheus.GaugeVec
	metricsNumOfActiveOrders *prometheus.GaugeVec
	metricsNumOfOpenOrders   *prometheus.GaugeVec
	metricsProfit            *prometheus.GaugeVec
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
	if metricsState == nil {
		metricsState = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bbgo_dca2_state",
				Help: "state of this DCA2 strategy",
			},
			append([]string{
				"exchange",
				"symbol",
			}, extendedLabels...),
		)
	}

	if metricsNumOfActiveOrders == nil {
		metricsNumOfActiveOrders = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bbgo_dca2_num_of_active_orders",
				Help: "number of active orders",
			},
			append([]string{
				"exchange",
				"symbol",
			}, extendedLabels...),
		)
	}

	if metricsNumOfOpenOrders == nil {
		metricsNumOfOpenOrders = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bbgo_dca2_num_of_open_orders",
				Help: "number of open orders",
			},
			append([]string{
				"exchange",
				"symbol",
			}, extendedLabels...),
		)
	}

	if metricsProfit == nil {
		metricsProfit = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bbgo_dca2_profit",
				Help: "profit of this DCA@ strategy",
			},
			append([]string{
				"exchange",
				"symbol",
				"round",
			}, extendedLabels...),
		)
	}
}

var metricsRegistered = false

func registerMetrics() {
	if metricsRegistered {
		return
	}

	initMetrics(nil)

	prometheus.MustRegister(
		metricsState,
		metricsNumOfActiveOrders,
		metricsNumOfOpenOrders,
		metricsProfit,
	)

	metricsRegistered = true
}

func updateProfitMetrics(round int64, profit float64) {
	labels := mergeLabels(baseLabels, prometheus.Labels{
		"round": strconv.FormatInt(round, 10),
	})
	metricsProfit.With(labels).Set(profit)
}
