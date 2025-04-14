package dca2

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsNumOfActiveOrders *prometheus.GaugeVec
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
	if metricsNumOfActiveOrders == nil {
		metricsNumOfActiveOrders = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bbgo_dca2_num_of_active_orders",
				Help: "number of active orders",
			},
			append([]string{
				"exchange",
				"symbol",
				"state",
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
		metricsNumOfActiveOrders,
	)

	metricsRegistered = true
}

func updateNumOfActiveOrdersMetrics(state State, numOfActiveOrders int64) {
	labels := mergeLabels(baseLabels, prometheus.Labels{
		"state": strconv.FormatInt(int64(state), 10),
	})
	metricsNumOfActiveOrders.With(labels).Set(float64(numOfActiveOrders))
}
