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
	// Threshold is the minimum absolute raw signal value to consider
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
		s.Threshold = 0.25
	}
	if s.WarmUpSampleCount == 0 {
		s.WarmUpSampleCount = 30
	}
	if s.StatsUpdateSampleCount == 0 {
		s.StatsUpdateSampleCount = 100
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

	// update stats within the sampling window
	slice := s.indicator.Slice.Tail(s.StatsUpdateSampleCount + 1)
	// exclude the last value
	lastIdx := len(slice) - 1
	slice.Pop(int64(lastIdx))
	// calculate moving mean and std and update the stats
	mean := slice.Mean()
	std := slice.Std()
	s.updateStats(mean, std)

	// note: s.std can be zero -> score could be ±Inf
	// use tanh to handle the ±Inf case
	// it's also possible where `last` == `mean` and `std` == 0 -> resulting in NaN score
	last := s.indicator.Last(0)
	score := (last - s.mean) / s.std
	sig := math.Tanh(score) * 2 // scale to [-2, 2]
	// if the sig is NaN or its absolute value is below the threshold, set it to zero -> reduce noise
	if math.IsNaN(sig) || math.Abs(sig) < s.Threshold {
		sig = 0.0
	}

	if s.logger != nil {
		s.logger.Infof("[LiquidityDemandSignal] last=%.2f, mean=%.2f, std=%.2f, score=%.2f, sig=%.2f", last, s.mean, s.std, score, sig)
	}
	return sig, nil
}

func (s *LiquidityDemandSignal) updateStats(mean, std float64) {
	if math.IsNaN(mean) || math.IsInf(mean, 0) {
		return
	}
	if math.IsNaN(std) || math.IsInf(std, 0) {
		return
	}
	// only update the stats if it's valid
	s.mean = mean
	s.std = std
}
