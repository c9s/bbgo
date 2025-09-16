package signal

import (
	"context"
	"errors"
	"math"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

var _ bbgo.SignalProvider = (*LiquidityDemandSignal)(nil)

type LiquidityDemandSignal struct {
	BaseProvider
	Logger

	types.IntervalWindow
	Threshold fixedpoint.Value `json:"threshold"`
	indicator *indicatorv2.LiquidityDemandStream
}

// bbgo.SignalProvider
func (s *LiquidityDemandSignal) ID() string {
	return "liquidityDemand"
}

func (s *LiquidityDemandSignal) Subscribe(session *bbgo.ExchangeSession, symbol string) {
	s.Symbol = symbol
	if s.Threshold.IsZero() {
		s.Threshold = fixedpoint.NewFromFloat(1000000)
	}
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: s.IntervalWindow.Interval})
}

func (s *LiquidityDemandSignal) Bind(_ context.Context, session *bbgo.ExchangeSession, symbol string) error {
	s.indicator = session.Indicators(symbol).LiquidityDemand(s.IntervalWindow)
	return nil
}

func (s *LiquidityDemandSignal) CalculateSignal(_ context.Context) (float64, error) {
	if s.indicator.Length() == 0 {
		return 0.0, errors.New("not enough data")
	}
	last := s.indicator.Last(0)
	lastF := fixedpoint.NewFromFloat(last)
	if fixedpoint.Abs(lastF) < s.Threshold {
		return 0.0, nil
	}
	sig := math.Tanh(last) * 2 // scale to [-2, 2]
	return sig, nil
}
