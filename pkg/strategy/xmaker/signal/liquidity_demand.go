package signal

import (
	"context"
	"math"

	"github.com/c9s/bbgo/pkg/bbgo"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

var _ bbgo.SignalProvider = (*LiquidityDemandSignal)(nil)

type LiquidityDemandSignal struct {
	BaseProvider
	Logger

	types.IntervalWindow
	Threshold              float64 `json:"threshold"`
	WarmUpSampleCount      int     `json:"warmUpSampleCount"`
	StatsUpdateSampleCount int     `json:"statsUpdateSampleCount"`

	indicator               *indicatorv2.LiquidityDemandStream
	statsUpdateSampleCountf float64
	mean, std               float64
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
	if s.WarmUpSampleCount == 0 {
		s.WarmUpSampleCount = 100
	}
	if s.StatsUpdateSampleCount == 0 {
		s.StatsUpdateSampleCount = 300
	}
	s.statsUpdateSampleCountf = float64(s.StatsUpdateSampleCount)
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: s.IntervalWindow.Interval})
}

func (s *LiquidityDemandSignal) Bind(_ context.Context, session *bbgo.ExchangeSession, symbol string) error {
	s.indicator = session.Indicators(symbol).LiquidityDemand(s.IntervalWindow)
	return nil
}

func (s *LiquidityDemandSignal) CalculateSignal(_ context.Context) (float64, error) {
	// still warming up
	if s.indicator.Length() <= s.WarmUpSampleCount {
		return 0.0, nil
	}
	// threshold check
	// if the last liquidity demand is less than the threshold, we consider it as a noise and return 0 (no signal)
	last := s.indicator.Last(0)
	if math.Abs(last) < s.Threshold {
		return 0.0, nil
	}
	// update mean and std by incremental calculation (Welford's online algorithm)
	if s.indicator.Length() <= s.StatsUpdateSampleCount {
		s.mean = s.indicator.Slice.Mean()
		s.std = s.indicator.Slice.Std()
	} else {
		s.mean = (s.mean*s.statsUpdateSampleCountf + last) / (s.statsUpdateSampleCountf + 1)
		d := last - s.mean
		s.std = math.Sqrt((s.std*s.std*s.statsUpdateSampleCountf + d*d) / (s.statsUpdateSampleCountf + 1))
	}
	// avoid division by zero
	if s.std == 0 {
		return 0.0, nil
	}
	score := (last - s.mean) / s.std
	sig := math.Tanh(score) * 2 // scale to [-2, 2]
	if math.IsNaN(sig) {
		sig = 0.0
	}
	if s.logger != nil {
		s.logger.Infof("[LiquidityDemandSignal] last=%.2f, mean=%.2f, std=%.2f, score=%.2f, sig=%.2f", last, s.mean, s.std, score, sig)
	}
	return sig, nil
}
