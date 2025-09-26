package xmaker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var positionExposurePendingMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_exposure_pending",
		Help: "the pending position exposure",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var positionExposureNetMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_exposure_net",
		Help: "the net position exposure",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var positionExposureUncoveredMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_position_exposure_uncovered",
		Help: "the uncovered position exposure",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var positionExposureSizeMetrics = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "bbgo_position_exposure_size",
		Help:    "the size of position exposure",
		Buckets: prometheus.LinearBuckets(0, 10, 10),
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

//go:generate callbackgen -type PositionExposure
type PositionExposure struct {
	symbol string

	// net = net position
	// pending = covered position
	net, pending fixedpoint.MutexValue

	openCallbacks  []func(d fixedpoint.Value)
	coverCallbacks []func(d fixedpoint.Value)
	closeCallbacks []func(d fixedpoint.Value)

	labels prometheus.Labels

	positionExposurePendingMetrics,
	positionExposureNetMetrics,
	positionExposureUncoveredMetrics prometheus.Gauge
	positionExposureSizeMetrics prometheus.Observer
}

func newPositionExposure(symbol string) *PositionExposure {
	return &PositionExposure{
		symbol: symbol,
	}
}

func (m *PositionExposure) Open(delta fixedpoint.Value) {
	m.net.Add(delta)

	log.Infof(
		"%s opened:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)

	m.EmitOpen(delta)
}

func (m *PositionExposure) Cover(delta fixedpoint.Value) {
	m.pending.Add(delta)

	log.Infof(
		"%s covered:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)

	m.EmitCover(delta)
}

func (m *PositionExposure) Close(delta fixedpoint.Value) {
	m.pending.Add(delta)
	m.net.Add(delta)

	log.Infof(
		"%s closed:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)

	m.EmitClose(delta)
}

func (m *PositionExposure) IsClosed() bool {
	return m.net.Get().IsZero() && m.pending.Get().IsZero()
}

func (m *PositionExposure) GetUncovered() fixedpoint.Value {
	netPosition := m.net.Get()
	coveredPosition := m.pending.Get()
	uncoverPosition := netPosition.Sub(coveredPosition)
	return uncoverPosition
}

func (m *PositionExposure) SetMetricsLabels(strategyType, strategyID, exchange, symbol string) {
	m.labels = prometheus.Labels{
		"strategy_type": strategyType,
		"strategy_id":   strategyID,
		"exchange":      exchange,
		"symbol":        symbol,
	}

	m.positionExposurePendingMetrics = positionExposurePendingMetrics.With(m.labels)
	m.positionExposureNetMetrics = positionExposureNetMetrics.With(m.labels)
	m.positionExposureUncoveredMetrics = positionExposureUncoveredMetrics.With(m.labels)
	m.positionExposureSizeMetrics = positionExposureSizeMetrics.With(m.labels)

	updater := func(delta fixedpoint.Value) {
		m.updateMetrics()
	}

	m.OnOpen(updater)
	m.OnCover(updater)
	m.OnClose(updater)
}

func (m *PositionExposure) updateMetrics() {
	m.positionExposurePendingMetrics.Set(m.pending.Get().Float64())
	m.positionExposureNetMetrics.Set(m.net.Get().Float64())
	m.positionExposureUncoveredMetrics.Set(m.GetUncovered().Float64())
	m.positionExposureSizeMetrics.Observe(m.net.Get().Float64())
}
