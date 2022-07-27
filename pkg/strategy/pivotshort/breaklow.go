package pivotshort

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/risk"
	"github.com/c9s/bbgo/pkg/types"
)

type StopEMA struct {
	types.IntervalWindow
	Range fixedpoint.Value `json:"range"`
}

type TrendEMA struct {
	types.IntervalWindow
}

type ClosedKLineStop struct {
	types.IntervalWindow
}

// BreakLow -- when price breaks the previous pivot low, we set a trade entry
type BreakLow struct {
	Symbol string
	Market types.Market
	types.IntervalWindow

	// Ratio is a number less than 1.0, price * ratio will be the price triggers the short order.
	Ratio fixedpoint.Value `json:"ratio"`

	// MarketOrder is the option to enable market order short.
	MarketOrder bool `json:"marketOrder"`

	// BounceRatio is a ratio used for placing the limit order sell price
	// limit sell price = breakLowPrice * (1 + BounceRatio)
	BounceRatio fixedpoint.Value `json:"bounceRatio"`

	Leverage fixedpoint.Value `json:"leverage"`
	Quantity fixedpoint.Value `json:"quantity"`

	StopEMA *StopEMA `json:"stopEMA"`

	TrendEMA *TrendEMA `json:"trendEMA"`

	ClosedKLineStop *ClosedKLineStop `json:"closedKLineStop"`

	lastLow fixedpoint.Value

	// lastBreakLow is the low that the price just break
	lastBreakLow fixedpoint.Value

	pivotLow       *indicator.PivotLow
	pivotLowPrices []fixedpoint.Value

	stopEWMA *indicator.EWMA

	trendEWMA                       *indicator.EWMA
	trendEWMALast, trendEWMACurrent float64

	orderExecutor *bbgo.GeneralOrderExecutor
	session       *bbgo.ExchangeSession
}

func (s *BreakLow) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})

	if s.StopEMA != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.StopEMA.Interval})
	}

	if s.TrendEMA != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.TrendEMA.Interval})
	}

	if s.ClosedKLineStop != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.ClosedKLineStop.Interval})
	}
}

func (s *BreakLow) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	symbol := position.Symbol
	standardIndicator := session.StandardIndicatorSet(s.Symbol)

	s.lastLow = fixedpoint.Zero

	s.pivotLow = standardIndicator.PivotLow(s.IntervalWindow)

	if s.StopEMA != nil {
		s.stopEWMA = standardIndicator.EWMA(s.StopEMA.IntervalWindow)
	}

	if s.TrendEMA != nil {
		s.trendEWMA = standardIndicator.EWMA(s.TrendEMA.IntervalWindow)

		session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.TrendEMA.Interval, func(kline types.KLine) {
			s.trendEWMALast = s.trendEWMACurrent
			s.trendEWMACurrent = s.trendEWMA.Last()
		}))
	}

	// update pivot low data
	session.MarketDataStream.OnStart(func() {
		if s.updatePivotLow() {
			bbgo.Notify("%s new pivot low: %f", s.Symbol, s.pivotLow.Last())
		}

		s.pilotQuantityCalculation()
	})

	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {
		if s.updatePivotLow() {
			// when position is opened, do not send pivot low notify
			if position.IsOpened(kline.Close) {
				return
			}

			bbgo.Notify("%s new pivot low: %f", s.Symbol, s.pivotLow.Last())
		}
	}))

	if s.ClosedKLineStop != nil {
		// if the position is already opened, and we just break the low, this checks if the kline closed above the low,
		// so that we can close the position earlier
		session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.ClosedKLineStop.Interval, func(k types.KLine) {
			// make sure the position is opened, and it's a short position
			if !position.IsOpened(k.Close) || !position.IsShort() {
				return
			}

			// make sure we recorded the last break low
			if s.lastBreakLow.IsZero() {
				return
			}

			// the kline opened below the last break low, and closed above the last break low
			if k.Open.Compare(s.lastBreakLow) < 0 && k.Close.Compare(s.lastBreakLow) > 0 {
				bbgo.Notify("kLine closed above the last break low, triggering stop earlier")
				if err := s.orderExecutor.ClosePosition(context.Background(), one, "kLineClosedStop"); err != nil {
					log.WithError(err).Error("position close error")
				}

				// reset to zero
				s.lastBreakLow = fixedpoint.Zero
			}
		}))
	}

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(kline types.KLine) {
		if len(s.pivotLowPrices) == 0 {
			log.Infof("currently there is no pivot low prices, can not check break low...")
			return
		}

		previousLow := s.pivotLowPrices[len(s.pivotLowPrices)-1]
		ratio := fixedpoint.One.Add(s.Ratio)
		breakPrice := previousLow.Mul(ratio)

		openPrice := kline.Open
		closePrice := kline.Close

		// if the previous low is not break, or the kline is not strong enough to break it, skip
		if closePrice.Compare(breakPrice) >= 0 {
			return
		}

		// we need the price cross the break line, or we do nothing:
		// open > break price > close price
		if !(openPrice.Compare(breakPrice) > 0 && closePrice.Compare(breakPrice) < 0) {
			return
		}

		// force direction to be down
		if closePrice.Compare(openPrice) >= 0 {
			log.Infof("%s price %f is closed higher than the open price %f, skip this break", kline.Symbol, closePrice.Float64(), openPrice.Float64())
			// skip UP klines
			return
		}

		log.Infof("%s breakLow signal detected, closed price %f < breakPrice %f", kline.Symbol, closePrice.Float64(), breakPrice.Float64())

		if s.lastBreakLow.IsZero() || previousLow.Compare(s.lastBreakLow) < 0 {
			s.lastBreakLow = previousLow
		}

		if position.IsOpened(kline.Close) {
			log.Infof("position is already opened, skip short")
			return
		}

		// trend EMA protection
		if s.trendEWMALast > 0.0 && s.trendEWMACurrent > 0.0 {
			slope := s.trendEWMALast / s.trendEWMACurrent
			if slope > 1.0 {
				log.Infof("trendEMA %+v current=%f last=%f slope=%f: skip short", s.TrendEMA, s.trendEWMACurrent, s.trendEWMALast, slope)
				return
			}

			log.Infof("trendEMA %+v current=%f last=%f slope=%f: short is enabled", s.TrendEMA, s.trendEWMACurrent, s.trendEWMALast, slope)
		}

		// stop EMA protection
		if s.stopEWMA != nil {
			ema := fixedpoint.NewFromFloat(s.stopEWMA.Last())
			if ema.IsZero() {
				return
			}

			emaStopShortPrice := ema.Mul(fixedpoint.One.Sub(s.StopEMA.Range))
			if closePrice.Compare(emaStopShortPrice) < 0 {
				log.Infof("stopEMA protection: close price %f < EMA(%v %f) * (1 - RANGE %f) = %f", closePrice.Float64(), s.StopEMA, ema.Float64(), s.StopEMA.Range.Float64(), emaStopShortPrice.Float64())
				return
			}
		}

		ctx := context.Background()

		// graceful cancel all active orders
		_ = orderExecutor.GracefulCancel(ctx)

		quantity, err := risk.CalculateBaseQuantity(s.session, s.Market, closePrice, s.Quantity, s.Leverage)
		if err != nil {
			log.WithError(err).Errorf("quantity calculation error")
		}

		if quantity.IsZero() {
			return
		}

		if s.MarketOrder {
			bbgo.Notify("%s price %f breaks the previous low %f with ratio %f, submitting market sell to open a short position", symbol, kline.Close.Float64(), previousLow.Float64(), s.Ratio.Float64())
			_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:           s.Symbol,
				Side:             types.SideTypeSell,
				Type:             types.OrderTypeMarket,
				Quantity:         quantity,
				MarginSideEffect: types.SideEffectTypeMarginBuy,
				Tag:              "breakLowMarket",
			})

		} else {
			sellPrice := previousLow.Mul(fixedpoint.One.Add(s.BounceRatio))

			bbgo.Notify("%s price %f breaks the previous low %f with ratio %f, submitting limit sell @ %f", symbol, kline.Close.Float64(), previousLow.Float64(), s.Ratio.Float64(), sellPrice.Float64())
			_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:           kline.Symbol,
				Side:             types.SideTypeSell,
				Type:             types.OrderTypeLimit,
				Price:            sellPrice,
				Quantity:         quantity,
				MarginSideEffect: types.SideEffectTypeMarginBuy,
				Tag:              "breakLowLimit",
			})
		}
	}))
}

func (s *BreakLow) pilotQuantityCalculation() {
	log.Infof("pilot calculation for max position: last low = %f, quantity = %f, leverage = %f",
		s.lastLow.Float64(),
		s.Quantity.Float64(),
		s.Leverage.Float64())

	quantity, err := risk.CalculateBaseQuantity(s.session, s.Market, s.lastLow, s.Quantity, s.Leverage)
	if err != nil {
		log.WithError(err).Errorf("quantity calculation error")
	}

	if quantity.IsZero() {
		log.WithError(err).Errorf("quantity is zero, can not submit order")
		return
	}

	bbgo.Notify("%s %f quantity will be used for shorting", s.Symbol, quantity.Float64())
}

func (s *BreakLow) updatePivotLow() bool {
	lastLow := fixedpoint.NewFromFloat(s.pivotLow.Last())
	if lastLow.IsZero() || lastLow.Compare(s.lastLow) == 0 {
		return false
	}

	s.lastLow = lastLow
	s.pivotLowPrices = append(s.pivotLowPrices, lastLow)
	return true
}
