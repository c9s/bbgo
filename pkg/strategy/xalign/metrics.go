package xalign

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var quantityDeltaMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xalign_balance_quantity_delta",
		Help: "The balance delta of the currency",
	},
	[]string{"currency", "side"},
)

var amountDeltaMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xalign_balance_amount_delta",
		Help: "The balance delta of the quote amount of the currency",
	},
	[]string{"currency", "side"},
)

func (s *Strategy) updateMetrics(currency string, side string, quantityDelta float64, amountDelta float64) {
	var quantityGauge, amountGauge prometheus.Gauge
	key := currency + "_" + side
	qKey := key + "_quantity"
	aKey := key + "_amount"
	if g, ok := s.gaugeMetrics[qKey]; ok {
		quantityGauge = g
	} else {
		quantityGauge = quantityDeltaMetrics.With(
			prometheus.Labels{
				"asset": currency,
				"side":  side,
			},
		)
		s.gaugeMetrics[qKey] = quantityGauge
	}
	if g, ok := s.gaugeMetrics[aKey]; ok {
		amountGauge = g
	} else {
		amountGauge = amountDeltaMetrics.With(
			prometheus.Labels{
				"asset": currency,
				"side":  side,
			},
		)
		s.gaugeMetrics[aKey] = amountGauge
	}

	quantityGauge.Set(quantityDelta)
	amountGauge.Set(amountDelta)
}
