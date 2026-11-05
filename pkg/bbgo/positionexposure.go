package bbgo

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Delta fixedpoint.Value

func (d Delta) Side() types.SideType {
	side := types.SideTypeBuy

	if fixedpoint.Value(d).IsZero() {
		side = types.SideTypeNone
	}

	if fixedpoint.Value(d).Sign() < 0 {
		side = types.SideTypeSell
	}

	return side
}

func (d Delta) Quantity() fixedpoint.Value {
	return fixedpoint.Value(d).Abs()
}

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

	// mu makes every update and read of net and pending one step. Close moves both,
	// and a reader between the two moves would see a half-applied delta: a covered
	// position closed by a fill would look uncovered and be hedged again.
	mu sync.RWMutex

	openCallbacks  []func(d fixedpoint.Value)
	coverCallbacks []func(d fixedpoint.Value)
	closeCallbacks []func(d fixedpoint.Value)

	labels prometheus.Labels

	positionExposurePendingMetrics,
	positionExposureNetMetrics,
	positionExposureUncoveredMetrics prometheus.Gauge
	positionExposureSizeMetrics prometheus.Observer

	logger logrus.FieldLogger
}

func NewPositionExposure(symbol string) *PositionExposure {
	return &PositionExposure{
		symbol: symbol,
		logger: logrus.WithField("symbol", symbol),
	}
}

func (m *PositionExposure) SetLogger(logger logrus.FieldLogger) {
	m.logger = logger
}

func (m *PositionExposure) GetSymbol() string {
	return m.symbol
}

func (m *PositionExposure) GetNet() fixedpoint.Value {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.net.Get()
}

func (m *PositionExposure) GetPending() fixedpoint.Value {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pending.Get()
}

// snapshot returns net and pending read together
func (m *PositionExposure) snapshot() (net, pending fixedpoint.Value) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.net.Get(), m.pending.Get()
}

// apply adds the deltas to net and pending in one step and returns the new values
func (m *PositionExposure) apply(netDelta, pendingDelta fixedpoint.Value) (net, pending fixedpoint.Value) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.net.Add(netDelta)
	m.pending.Add(pendingDelta)
	return m.net.Get(), m.pending.Get()
}

func (m *PositionExposure) Open(delta fixedpoint.Value) {
	net, pending := m.apply(delta, fixedpoint.Zero)

	m.logger.Infof(
		"%s opened:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		net.Float64(),
		pending.Float64(),
	)

	m.EmitOpen(delta)
}

func (m *PositionExposure) Uncover(delta fixedpoint.Value) {
	delta = delta.Neg()

	net, pending := m.apply(fixedpoint.Zero, delta)

	m.logger.Infof(
		"%s uncovered:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		net.Float64(),
		pending.Float64(),
	)

	m.EmitCover(delta)
}

func (m *PositionExposure) Cover(delta fixedpoint.Value) {
	net, pending := m.apply(fixedpoint.Zero, delta)

	m.logger.Infof(
		"%s covered:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		net.Float64(),
		pending.Float64(),
	)

	m.EmitCover(delta)
}

func (m *PositionExposure) Close(delta fixedpoint.Value) {
	net, pending := m.apply(delta, delta)

	m.logger.Infof(
		"%s closed:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		net.Float64(),
		pending.Float64(),
	)

	m.EmitClose(delta)
}

func (m *PositionExposure) IsClosed() bool {
	net, pending := m.snapshot()
	return net.IsZero() && pending.IsZero()
}

func (m *PositionExposure) String() string {
	net, pending := m.snapshot()
	return fmt.Sprintf("PositionExposure<%s> net:%s pending:%s uncovered:%s",
		m.symbol,
		net.String(),
		pending.String(),
		net.Sub(pending).String(),
	)
}

func (m *PositionExposure) GetUncovered() fixedpoint.Value {
	netPosition, coveredPosition := m.snapshot()
	return netPosition.Sub(coveredPosition)
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
	net, pending := m.snapshot()
	m.positionExposurePendingMetrics.Set(pending.Float64())
	m.positionExposureNetMetrics.Set(net.Float64())
	m.positionExposureUncoveredMetrics.Set(net.Sub(pending).Float64())
	m.positionExposureSizeMetrics.Observe(net.Float64())
}
