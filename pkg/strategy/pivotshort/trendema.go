package pivotshort

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type TrendEMA struct {
	types.IntervalWindow

	// MaxGradient is the maximum gradient allowed for the entry.
	MaxGradient float64 `json:"maxGradient"`
	MinGradient float64 `json:"minGradient"`

	trendEWMA *indicator.EWMA

	trendEWMALast, trendEWMACurrent, trendGradient float64
}

func (s *TrendEMA) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	symbol := orderExecutor.Position().Symbol
	s.trendEWMA = session.StandardIndicatorSet(symbol).EWMA(s.IntervalWindow)
	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {
		s.trendEWMALast = s.trendEWMACurrent
		s.trendEWMACurrent = s.trendEWMA.Last()
	}))
}

func (s *TrendEMA) GradientAllowed() bool {
	if s.trendEWMALast > 0.0 && s.trendEWMACurrent > 0.0 {
		s.trendGradient = s.trendEWMALast / s.trendEWMACurrent
	}

	log.Infof("trendEMA %+v current=%f last=%f gradient=%f", s, s.trendEWMACurrent, s.trendEWMALast, s.trendGradient)

	if s.trendGradient == .0 {
		return false
	}

	if s.MaxGradient > 0.0 && s.trendGradient < s.MaxGradient {
		return true
	}

	if s.MinGradient > 0.0 && s.trendGradient > s.MinGradient {
		return true
	}

	return false
}
