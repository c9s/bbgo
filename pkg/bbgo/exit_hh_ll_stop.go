package bbgo

import (
	"context"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TODO: if parameter not set
// TODO: log and notify
// TODO: check all procedures

type HigherHighLowerLowStop struct {
	Symbol string `json:"symbol"`

	types.IntervalWindow

	HighLowWindow int `json:"highLowWindow"`

	MaxHighLow int `json:"maxHighLow"`

	MinHighLow int `json:"minHighLow"`

	// ActivationRatio is the trigger condition
	// When the price goes higher (lower for short position) than this ratio, the stop will be activated.
	// You can use this to combine several exits
	ActivationRatio fixedpoint.Value `json:"activationRatio"`

	// DeactivationRatio is the kill condition
	// When the price goes higher (lower for short position) than this ratio, the stop will be deactivated.
	// You can use this to combine several exits
	DeactivationRatio fixedpoint.Value `json:"deactivationRatio"`

	OppositeDirectionAsPosition bool `json:"oppositeDirectionAsPosition"`

	klines types.KLineWindow

	// activated: when the price reaches the min profit price, we set the activated to true to enable trailing stop
	activated bool

	highLows []types.Direction

	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

// Subscribe required k-line stream
func (s *HigherHighLowerLowStop) Subscribe(session *ExchangeSession) {
	// use 1m kline to handle roi stop
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

// updateActivated checks the position cost against the close price, activation ratio, and deactivation ratio to
// determine whether this stop should be activated
func (s *HigherHighLowerLowStop) updateActivated(position *types.Position, closePrice fixedpoint.Value) {
	if position.IsClosed() || position.IsDust(closePrice) {
		s.activated = false
	} else if s.activated {
		if position.IsLong() {
			r := fixedpoint.One.Add(s.DeactivationRatio)
			if closePrice.Compare(position.AverageCost.Mul(r)) >= 0 {
				s.activated = false
				Notify("[hhllStop] Stop of %s deactivated for long position, deactivation ratio %f:", s.Symbol, s.DeactivationRatio)
			}
		} else if position.IsShort() {
			r := fixedpoint.One.Sub(s.DeactivationRatio)
			// for short position, if the close price is less than the activation price then this is a profit position.
			if closePrice.Compare(position.AverageCost.Mul(r)) <= 0 {
				s.activated = false
				Notify("[hhllStop] Stop of %s deactivated for short position, deactivation ratio %f:", s.Symbol, s.DeactivationRatio)
			}
		}
	} else {
		if position.IsLong() {
			r := fixedpoint.One.Add(s.ActivationRatio)
			if closePrice.Compare(position.AverageCost.Mul(r)) >= 0 {
				s.activated = true
				Notify("[hhllStop] Stop of %s activated for long position, activation ratio %f:", s.Symbol, s.ActivationRatio)
			}
		} else if position.IsShort() {
			r := fixedpoint.One.Sub(s.ActivationRatio)
			// for short position, if the close price is less than the activation price then this is a profit position.
			if closePrice.Compare(position.AverageCost.Mul(r)) <= 0 {
				s.activated = true
				Notify("[hhllStop] Stop of %s activated for short position, activation ratio %f:", s.Symbol, s.ActivationRatio)
			}
		}
	}
}

func (s *HigherHighLowerLowStop) updateHighLowNumber(kline types.KLine) {
	if !s.activated {
		s.reset()
		return
	}

	if s.klines.Len() > 0 {
		if s.klines.GetHigh().Compare(kline.GetHigh()) < 0 {
			s.highLows = append(s.highLows, types.DirectionUp)
		} else if s.klines.GetLow().Compare(kline.GetLow()) > 0 {
			s.highLows = append(s.highLows, types.DirectionDown)
		} else {
			s.highLows = append(s.highLows, types.DirectionNone)
		}
		// Truncate highLows
		if len(s.highLows) > s.HighLowWindow {
			end := len(s.highLows)
			start := end - s.HighLowWindow
			if start < 0 {
				start = 0
			}
			kn := s.highLows[start:]
			s.highLows = kn
		}
	} else {
		s.highLows = append(s.highLows, types.DirectionNone)
	}

	s.klines.Add(kline)
	s.klines.Truncate(s.Window - 1)
}

func (s *HigherHighLowerLowStop) shouldStop(position *types.Position) bool {
	if s.activated {
		highs := 0
		lows := 0
		for _, hl := range s.highLows {
			switch hl {
			case types.DirectionUp:
				highs++
			case types.DirectionDown:
				lows++
			}
		}

		log.Debugf("[hhllStop] %d higher highs and %d lower lows in window of %d", highs, lows, s.HighLowWindow)

		// Check higher highs
		if (position.IsLong() && !s.OppositeDirectionAsPosition) || (position.IsShort() && s.OppositeDirectionAsPosition) {
			if (s.MinHighLow > 0 && highs < s.MinHighLow) || (s.MaxHighLow > 0 && highs > s.MaxHighLow) {
				return true
			}
			// Check lower lows
		} else if (position.IsShort() && !s.OppositeDirectionAsPosition) || (position.IsLong() && s.OppositeDirectionAsPosition) {
			if (s.MinHighLow > 0 && lows < s.MinHighLow) || (s.MaxHighLow > 0 && lows > s.MaxHighLow) {
				return true
			}
		}
	}
	return false
}

func (s *HigherHighLowerLowStop) reset() {
	s.highLows = []types.Direction{}
	s.klines.Truncate(0)
}

func (s *HigherHighLowerLowStop) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		s.updateActivated(position, kline.GetClose())

		s.updateHighLowNumber(kline)

		// Close position & reset
		if s.activated && s.shouldStop(position) {
			err := s.orderExecutor.ClosePosition(context.Background(), fixedpoint.One, "hhllStop")
			if err != nil {
				Notify("[hhllStop] Stop of %s triggered but failed to close %s position:", s.Symbol, err)
			} else {
				s.reset()
				s.activated = false
				Notify("[hhllStop] Stop of %s triggered and position closed", s.Symbol)
			}
		}
	}))

	// Make sure the stop is reset when position is closed or dust
	orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		if position.IsClosed() || position.IsDust(position.AverageCost) {
			s.reset()
			s.activated = false
		}
	})
}
