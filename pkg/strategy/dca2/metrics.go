package dca2

import (
	"github.com/c9s/bbgo/pkg/strategy/dca2/statemachine"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsNumOfActiveOrders *prometheus.GaugeVec
	metricsState             *prometheus.GaugeVec
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
			}, extendedLabels...),
		)
	}

	if metricsState == nil {
		metricsState = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bbgo_dca2_state",
				Help: "state of this strategy",
			},
			append([]string{
				"exchange",
				"symbol",
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
		metricsState,
	)

	metricsRegistered = true
}

func updateNumOfActiveOrdersMetrics(numOfActiveOrders int) {
	// use ts to bind the state and numOfActiveOrders
	labels := mergeLabels(baseLabels, prometheus.Labels{
		// "ts": strconv.FormatInt(now.UnixMilli(), 10),
	})

	metricsNumOfActiveOrders.With(labels).Set(float64(numOfActiveOrders))
}

func updateStatsMetrics(state statemachine.State) {
	// use ts to bind the state and numOfActiveOrders
	labels := mergeLabels(baseLabels, prometheus.Labels{
		// "ts": strconv.FormatInt(now.UnixMilli(), 10),
	})

	metricsState.With(labels).Set(float64(state))
}
