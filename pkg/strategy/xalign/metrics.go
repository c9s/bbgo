package xalign

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type deltaGaugeKey struct {
	currency string
	side     string
	typ      string
}

var quantityDeltaMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xalign_balance_quantity_delta",
		Help: "The balance delta of the currency in quantity",
	},
	[]string{"currency", "side"},
)

var amountDeltaMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xalign_balance_value_delta",
		Help: "The balance delta of the currency in quote value",
	},
	[]string{"currency", "side"},
)

func (s *Strategy) updateMetrics(currency string, side string, quantityDelta float64, valueDelta float64) {
	var quantityGauge, valueGauge prometheus.Gauge
	qKey := deltaGaugeKey{
		currency: currency,
		side:     side,
		typ:      "quantity",
	}
	vKey := deltaGaugeKey{
		currency: currency,
		side:     side,
		typ:      "value",
	}
	if g, ok := s.deltaGaugesMap[qKey]; ok {
		quantityGauge = g
	} else {
		quantityGauge = quantityDeltaMetrics.With(
			prometheus.Labels{
				"currency": currency,
				"side":     side,
			},
		)
		s.deltaGaugesMap[qKey] = quantityGauge
	}
	if g, ok := s.deltaGaugesMap[vKey]; ok {
		valueGauge = g
	} else {
		valueGauge = amountDeltaMetrics.With(
			prometheus.Labels{
				"currency": currency,
				"side":     side,
			},
		)
		s.deltaGaugesMap[vKey] = valueGauge
	}

	quantityGauge.Set(quantityDelta)
	valueGauge.Set(valueDelta)
}
