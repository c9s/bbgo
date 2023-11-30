package bbgo

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type HigherHighLowerLowStop struct {
	Symbol string `json:"symbol"`

	// Interval is the kline interval used by this exit. Window is used as the range to determining higher highs and
	// lower lows
	types.IntervalWindow

	// HighLowWindow is the range to calculate the number of higher highs and lower lows
	HighLowWindow int `json:"highLowWindow"`

	// If the number of higher highs or lower lows with in HighLowWindow is more than MaxHighLow, the exit is triggered.
	// 0 disables this parameter. Either one of MaxHighLow and MinHighLow must be larger than 0
	MaxHighLow int `json:"maxHighLow"`

	// If the number of higher highs or lower lows with in HighLowWindow is less than MinHighLow, the exit is triggered.
	// 0 disables this parameter. Either one of MaxHighLow and MinHighLow must be larger than 0
	MinHighLow int `json:"minHighLow"`

	// ActivationRatio is the trigger condition
	// When the price goes higher (lower for short position) than this ratio, the stop will be activated.
	// You can use this to combine several exits
	ActivationRatio fixedpoint.Value `json:"activationRatio"`

	// DeactivationRatio is the kill condition
	// When the price goes higher (lower for short position) than this ratio, the stop will be deactivated.
	// You can use this to combine several exits
	DeactivationRatio fixedpoint.Value `json:"deactivationRatio"`

	// If true, looking for lower lows in long position and higher highs in short position. If false, looking for higher
	// highs in long position and lower lows in short position
	OppositeDirectionAsPosition bool `json:"oppositeDirectionAsPosition"`

	klines types.KLineWindow

	// activated: when the price reaches the min profit price, we set the activated to true to enable hhll stop
	activated bool

	highLows []types.Direction

	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor
}

// Subscribe required k-line stream
func (s *HigherHighLowerLowStop) Subscribe(session *ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

// updateActivated checks the position cost against the close price, activation ratio, and deactivation ratio to
// determine whether this stop should be activated
func (s *HigherHighLowerLowStop) updateActivated(position *types.Position, closePrice fixedpoint.Value) {
	// deactivate when no position
	if position.IsClosed() || position.IsDust(closePrice) || position.IsClosing() {

		s.activated = false
		return

	}

	// activation/deactivation price
	var priceDeactive fixedpoint.Value
	var priceActive fixedpoint.Value
	if position.IsLong() {
		priceDeactive = position.AverageCost.Mul(fixedpoint.One.Add(s.DeactivationRatio))
		priceActive = position.AverageCost.Mul(fixedpoint.One.Add(s.ActivationRatio))
	} else {
		priceDeactive = position.AverageCost.Mul(fixedpoint.One.Sub(s.DeactivationRatio))
		priceActive = position.AverageCost.Mul(fixedpoint.One.Sub(s.ActivationRatio))
	}

	if s.activated {

		if position.IsLong() {

			if closePrice.Compare(priceDeactive) >= 0 {

				s.activated = false
				Notify("[hhllStop] Stop of %s deactivated for long position, deactivation ratio %s", s.Symbol, s.DeactivationRatio.Percentage())

			} else if closePrice.Compare(priceActive) < 0 {

				s.activated = false
				Notify("[hhllStop] Stop of %s deactivated for long position, activation ratio %s", s.Symbol, s.ActivationRatio.Percentage())

			}

		} else if position.IsShort() {

			// for short position, if the close price is less than the activation price then this is a profit position.
			if closePrice.Compare(priceDeactive) <= 0 {

				s.activated = false
				Notify("[hhllStop] Stop of %s deactivated for short position, deactivation ratio %s", s.Symbol, s.DeactivationRatio.Percentage())

			} else if closePrice.Compare(priceActive) > 0 {

				s.activated = false
				Notify("[hhllStop] Stop of %s deactivated for short position, activation ratio %s", s.Symbol, s.ActivationRatio.Percentage())

			}

		}
	} else {

		if position.IsLong() {

			if closePrice.Compare(priceActive) >= 0 && closePrice.Compare(priceDeactive) < 0 {

				s.activated = true
				Notify("[hhllStop] %s stop is activated for long position, activation ratio %s, deactivation ratio %s", s.Symbol, s.ActivationRatio.Percentage(), s.DeactivationRatio.Percentage())

			}

		} else if position.IsShort() {

			// for short position, if the close price is less than the activation price then this is a profit position.
			if closePrice.Compare(priceActive) <= 0 && closePrice.Compare(priceDeactive) > 0 {

				s.activated = true
				Notify("[hhllStop] %s stop is activated for short position, activation ratio %s, deactivation ratio %s", s.Symbol, s.ActivationRatio.Percentage(), s.DeactivationRatio.Percentage())

			}

		}

	}
}

func (s *HigherHighLowerLowStop) updateHighLowNumber(kline types.KLine) {
	s.klines.Truncate(s.Window - 1)

	if s.klines.Len() >= s.Window-1 {
		high := kline.GetHigh()
		low := kline.GetLow()
		if s.klines.GetHigh().Compare(high) < 0 {
			s.highLows = append(s.highLows, types.DirectionUp)
			log.Debugf("[hhllStop] detected %s new higher high %f", s.Symbol, high.Float64())
		} else if s.klines.GetLow().Compare(low) > 0 {
			s.highLows = append(s.highLows, types.DirectionDown)
			log.Debugf("[hhllStop] detected %s new lower low %f", s.Symbol, low.Float64())
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
}

func (s *HigherHighLowerLowStop) shouldStop(position *types.Position) bool {
	if s.klines.Len() < s.Window || len(s.highLows) < s.HighLowWindow {
		log.Debugf("[hhllStop] not enough data for %s yet", s.Symbol)
		return false
	}

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

func (s *HigherHighLowerLowStop) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	// Check parameters
	if s.Window <= 0 {
		panic(fmt.Errorf("[hhllStop] window must be larger than zero"))
	}
	if s.HighLowWindow <= 0 {
		panic(fmt.Errorf("[hhllStop] highLowWindow must be larger than zero"))
	}
	if s.MaxHighLow <= 0 && s.MinHighLow <= 0 {
		panic(fmt.Errorf("[hhllStop] either maxHighLow or minHighLow must be larger than zero"))
	}

	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		s.updateActivated(position, kline.GetClose())

		s.updateHighLowNumber(kline)

		// Close position & reset
		if s.shouldStop(position) {
			defer func() {
				s.activated = false
			}()

			err := s.orderExecutor.ClosePosition(context.Background(), fixedpoint.One, "hhllStop")
			if err != nil {
				Notify("[hhllStop] Stop of %s triggered but failed to close %s position:", s.Symbol, err)
				return
			}

			Notify("[hhllStop] Stop of %s triggered and position closed", s.Symbol)
		}
	}))

	// Make sure the stop is reset when position is closed or dust
	orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		if position.IsClosed() || position.IsDust(position.AverageCost) {
			s.activated = false
		}
	})
}
