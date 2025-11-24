package common

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var positionMetrics = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_strategy_position",
		Help: "The current position of the strategy",
	},
	[]string{"strategy", "instance", "symbol"},
)

type PositionProvider interface {
	ID() string
	InstanceID() string
	GetSymbol() string
	GetPosition() fixedpoint.Value
}

type PositionTracker struct {
	provider PositionProvider
	metric   prometheus.Gauge

	interval types.Interval
}

func NewPositionTracker(positionProvider PositionProvider, interval types.Interval) *PositionTracker {
	id := positionProvider.ID()
	instanceID := positionProvider.InstanceID()
	symbol := positionProvider.GetSymbol()

	return &PositionTracker{
		provider: positionProvider,
		metric:   positionMetrics.WithLabelValues(id, instanceID, symbol),
		interval: interval,
	}
}

func (t *PositionTracker) Run(ctx context.Context) error {
	ticker := time.NewTicker(t.interval.Duration())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			position := t.provider.GetPosition()
			t.update(position)
		}
	}
}

func (t *PositionTracker) update(position fixedpoint.Value) {
	t.metric.Set(position.Float64())
}
