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
	qKey := deltaGaugeKey{
		currency: currency,
		side:     side,
		typ:      "quantity",
	}
	aKey := deltaGaugeKey{
		currency: currency,
		side:     side,
		typ:      "amount",
	}
	if g, ok := s.deltaGaugesMap[qKey]; ok {
		quantityGauge = g
	} else {
		quantityGauge = quantityDeltaMetrics.With(
			prometheus.Labels{
				"asset": currency,
				"side":  side,
			},
		)
		s.deltaGaugesMap[qKey] = quantityGauge
	}
	if g, ok := s.deltaGaugesMap[aKey]; ok {
		amountGauge = g
	} else {
		amountGauge = amountDeltaMetrics.With(
			prometheus.Labels{
				"asset": currency,
				"side":  side,
			},
		)
		s.deltaGaugesMap[aKey] = amountGauge
	}

	quantityGauge.Set(quantityDelta)
	amountGauge.Set(amountDelta)
}
