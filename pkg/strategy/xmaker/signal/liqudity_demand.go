package signal

import (
	"context"
	"math"

	"github.com/c9s/bbgo/pkg/bbgo"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

var _ bbgo.SignalProvider = (*LiquidityDemandSignal)(nil)

const (
	warmUpInterval       = 100
	statsUpdateInterval  = 300
	statsUpdateIntervalF = float64(statsUpdateInterval)
)

type LiquidityDemandSignal struct {
	BaseProvider
	Logger

	types.IntervalWindow
	Threshold float64 `json:"threshold"`
	indicator *indicatorv2.LiquidityDemandStream
	mean, std float64
}

// bbgo.SignalProvider
func (s *LiquidityDemandSignal) ID() string {
	return "liquidityDemand"
}

func (s *LiquidityDemandSignal) Subscribe(session *bbgo.ExchangeSession, symbol string) {
	s.Symbol = symbol
	if s.Threshold == 0 {
		s.Threshold = 1000000.0
	}
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: s.IntervalWindow.Interval})
}

func (s *LiquidityDemandSignal) Bind(_ context.Context, session *bbgo.ExchangeSession, symbol string) error {
	s.indicator = session.Indicators(symbol).LiquidityDemand(s.IntervalWindow)
	return nil
}

func (s *LiquidityDemandSignal) CalculateSignal(_ context.Context) (float64, error) {
	// still warming up
	if s.indicator.Length() <= warmUpInterval {
		return 0.0, nil
	}
	// threshold check
	// if the last liquidity demand is less than the threshold, we consider it as a noise and return 0 (no signal)
	last := s.indicator.Last(0)
	if math.Abs(last) < s.Threshold {
		return 0.0, nil
	}
	// update mean and std by incremental calculation (Welford's online algorithm)
	if s.indicator.Length() <= statsUpdateInterval {
		s.mean = s.indicator.Slice.Mean()
		s.std = s.indicator.Slice.Std()
	} else {
		s.mean = (s.mean*statsUpdateIntervalF + s.indicator.Last(0)) / (statsUpdateIntervalF + 1)
		d := last - s.mean
		s.std = math.Sqrt((s.std*s.std*statsUpdateIntervalF + d*d) / (statsUpdateIntervalF + 1))
	}
	// avoid division by zero
	if s.std == 0 {
		return 0.0, nil
	}
	score := (last - s.mean) / s.std
	sig := math.Tanh(score) * 2 // scale to [-2, 2]
	return sig, nil
}
