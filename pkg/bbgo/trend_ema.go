package bbgo

import (
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type TrendEMA struct {
	types.IntervalWindow

	// MaxGradient is the maximum gradient allowed for the entry.
	MaxGradient float64 `json:"maxGradient"`
	MinGradient float64 `json:"minGradient"`

	ewma *indicator.EWMA

	last, current float64
}

func (s *TrendEMA) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	symbol := orderExecutor.Position().Symbol
	s.ewma = session.StandardIndicatorSet(symbol).EWMA(s.IntervalWindow)

	session.MarketDataStream.OnStart(func() {
		if s.ewma.Length() < 2 {
			return
		}

		s.last = s.ewma.Values[s.ewma.Length()-2]
		s.current = s.ewma.Last(0)
	})

	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {
		s.last = s.current
		s.current = s.ewma.Last(0)
	}))
}

func (s *TrendEMA) Gradient() float64 {
	if s.last > 0.0 && s.current > 0.0 {
		return s.current / s.last
	}
	return 0.0
}

func (s *TrendEMA) GradientAllowed() bool {
	gradient := s.Gradient()

	logrus.Infof("trendEMA %+v current=%f last=%f gradient=%f", s, s.current, s.last, gradient)

	if gradient == .0 {
		return false
	}

	if s.MaxGradient > 0.0 && gradient > s.MaxGradient {
		return false
	}

	if s.MinGradient > 0.0 && gradient < s.MinGradient {
		return false
	}

	return true
}
