package signal

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

var _ bbgo.SignalProvider = (*PremiumSignal)(nil)

type PremiumSignal struct {
	BaseProvider
	Logger

	types.IntervalWindow

	Margin  float64 `json:"margin"`
	SymbolA string  `json:"symbolA"`
	SymbolB string  `json:"symbolB"`

	premiumStream *indicatorv2.PremiumStream
}

func (s *PremiumSignal) ID() string {
	return "price-premium"
}

func (s *PremiumSignal) Subscribe(session *bbgo.ExchangeSession, _ string) {
	session.Subscribe(types.KLineChannel, s.SymbolA, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.SymbolB, types.SubscribeOptions{Interval: s.Interval})
}

func (s *PremiumSignal) Bind(_ context.Context, session *bbgo.ExchangeSession, _ string) error {
	priceStreamA := session.Indicators(s.SymbolA).CLOSE(s.Interval)
	priceStreamB := session.Indicators(s.SymbolB).CLOSE(s.Interval)
	s.premiumStream = indicatorv2.Premium(
		priceStreamA,
		priceStreamB,
		s.Margin,
	)
	return nil
}

func (s *PremiumSignal) CalculateSignal(_ context.Context) (float64, error) {
	if s.premiumStream.Length() == 0 {
		return 0.0, nil
	}
	last := s.premiumStream.Last(0)
	if s.logger != nil {
		s.logger.Debugf("premium last raw value: %f", last)
	}
	if last == 1.0 {
		return 0.0, nil
	}
	// TODO: making the signal strength proportional to the premium value and adaptive
	var sig float64
	if last > 1.0 {
		sig = 1.0
	} else {
		sig = -1.0
	}
	return sig, nil
}
