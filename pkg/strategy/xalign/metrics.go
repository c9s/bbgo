package xalign

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type deltaGaugeKey struct {
	asset string
	side  string
	typ   string
}

var quantityDeltaMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xalign_balance_quantity_delta",
		Help: "The balance delta of the asset",
	},
	[]string{"asset", "side"},
)

var amountDeltaMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "xalign_balance_amount_delta",
		Help: "The balance delta of the quote amount of the asset",
	},
	[]string{"asset", "side"},
)

func (s *Strategy) updateMetrics(asset string, side string, quantityDelta float64, amountDelta float64) {
	var quantityGauge, amountGauge prometheus.Gauge
	qKey := deltaGaugeKey{
		asset: asset,
		side:  side,
		typ:   "quantity",
	}
	aKey := deltaGaugeKey{
		asset: asset,
		side:  side,
		typ:   "amount",
	}
	if g, ok := s.deltaGaugesMap[qKey]; ok {
		quantityGauge = g
	} else {
		quantityGauge = quantityDeltaMetrics.With(
			prometheus.Labels{
				"asset": asset,
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
				"asset": asset,
				"side":  side,
			},
		)
		s.deltaGaugesMap[aKey] = amountGauge
	}

	quantityGauge.Set(quantityDelta)
	amountGauge.Set(amountDelta)
}
