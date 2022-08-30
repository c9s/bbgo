package pivotshort

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/risk"
	"github.com/c9s/bbgo/pkg/types"
)

// FailedBreakHigh -- when price breaks the previous pivot low, we set a trade entry
type FailedBreakHigh struct {
	Symbol string
	Market types.Market
	types.IntervalWindow

	Enabled bool `json:"enabled"`

	// Ratio is a number less than 1.0, price * ratio will be the price triggers the short order.
	Ratio fixedpoint.Value `json:"ratio"`

	// MarketOrder is the option to enable market order short.
	MarketOrder bool `json:"marketOrder"`

	Leverage fixedpoint.Value `json:"leverage"`
	Quantity fixedpoint.Value `json:"quantity"`

	StopEMA *bbgo.StopEMA `json:"stopEMA"`

	TrendEMA *bbgo.TrendEMA `json:"trendEMA"`

	lastFailedBreakHigh, lastHigh fixedpoint.Value

	pivotHigh       *indicator.PivotHigh
	PivotHighPrices []fixedpoint.Value

	orderExecutor *bbgo.GeneralOrderExecutor
	session       *bbgo.ExchangeSession
}

func (s *FailedBreakHigh) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})

	if s.StopEMA != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.StopEMA.Interval})
	}

	if s.TrendEMA != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.TrendEMA.Interval})
	}
}

func (s *FailedBreakHigh) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	if !s.Enabled {
		return
	}

	position := orderExecutor.Position()
	symbol := position.Symbol
	standardIndicator := session.StandardIndicatorSet(s.Symbol)

	s.lastHigh = fixedpoint.Zero
	s.pivotHigh = standardIndicator.PivotHigh(s.IntervalWindow)

	if s.StopEMA != nil {
		s.StopEMA.Bind(session, orderExecutor)
	}

	if s.TrendEMA != nil {
		s.TrendEMA.Bind(session, orderExecutor)
	}

	// update pivot low data
	session.MarketDataStream.OnStart(func() {
		if s.updatePivotHigh() {
			bbgo.Notify("%s new pivot high: %f", s.Symbol, s.pivotHigh.Last())
		}

		s.pilotQuantityCalculation()
	})

	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {
		if s.updatePivotHigh() {
			// when position is opened, do not send pivot low notify
			if position.IsOpened(kline.Close) {
				return
			}

			bbgo.Notify("%s new pivot low: %f", s.Symbol, s.pivotHigh.Last())
		}
	}))

	// if the position is already opened, and we just break the low, this checks if the kline closed above the low,
	// so that we can close the position earlier
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(k types.KLine) {
		if !s.Enabled {
			return
		}

		// make sure the position is opened, and it's a short position
		if !position.IsOpened(k.Close) || !position.IsShort() {
			return
		}

		// make sure we recorded the last break low
		if s.lastFailedBreakHigh.IsZero() {
			return
		}

		// the kline opened below the last break low, and closed above the last break low
		if k.Open.Compare(s.lastFailedBreakHigh) < 0 && k.Close.Compare(s.lastFailedBreakHigh) > 0 {
			bbgo.Notify("kLine closed above the last break low, triggering stop earlier")
			if err := s.orderExecutor.ClosePosition(context.Background(), one, "fakeBreakStop"); err != nil {
				log.WithError(err).Error("position close error")
			}

			// reset to zero
			s.lastFailedBreakHigh = fixedpoint.Zero
		}
	}))

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(kline types.KLine) {
		if len(s.PivotHighPrices) == 0 || s.lastHigh.IsZero() {
			log.Infof("currently there is no pivot high prices, can not check failed break high...")
			return
		}

		previousHigh := s.lastHigh
		ratio := fixedpoint.One.Add(s.Ratio)
		breakPrice := previousHigh.Mul(ratio)

		openPrice := kline.Open
		closePrice := kline.Close

		// we need few conditions:
		// 1) kline.High is higher than the previous high
		// 2) kline.Close is lower than the previous high
		// 3) kline.Close is lower than kline.Open
		if kline.High.Compare(breakPrice) < 0 || closePrice.Compare(breakPrice) >= 0 {
			return
		}

		if closePrice.Compare(openPrice) > 0 {
			bbgo.Notify("the closed price is higher than the open price, skip failed break high short")
			return
		}

		bbgo.Notify("%s FailedBreakHigh signal detected, closed price %f < breakPrice %f", kline.Symbol, closePrice.Float64(), breakPrice.Float64())

		if s.lastFailedBreakHigh.IsZero() || previousHigh.Compare(s.lastFailedBreakHigh) < 0 {
			s.lastFailedBreakHigh = previousHigh
		}

		if position.IsOpened(kline.Close) {
			bbgo.Notify("position is already opened, skip")
			return
		}

		// trend EMA protection
		if s.TrendEMA != nil && !s.TrendEMA.GradientAllowed() {
			bbgo.Notify("trendEMA protection: close price %f, gradient %f", kline.Close.Float64(), s.TrendEMA.Gradient())
			return
		}

		// stop EMA protection
		if s.StopEMA != nil {
			if !s.StopEMA.Allowed(closePrice) {
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
			log.Warn("quantity is zero, can not submit order, skip")
			return
		}

		if s.MarketOrder {
			bbgo.Notify("%s price %f failed breaking the previous high %f with ratio %f, submitting market sell to open a short position", symbol, kline.Close.Float64(), previousHigh.Float64(), s.Ratio.Float64())
			_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:           s.Symbol,
				Side:             types.SideTypeSell,
				Type:             types.OrderTypeMarket,
				Quantity:         quantity,
				MarginSideEffect: types.SideEffectTypeMarginBuy,
				Tag:              "FailedBreakHighMarket",
			})

		} else {
			sellPrice := previousHigh

			bbgo.Notify("%s price %f failed breaking the previous high %f with ratio %f, submitting limit sell @ %f", symbol, kline.Close.Float64(), previousHigh.Float64(), s.Ratio.Float64(), sellPrice.Float64())
			_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:           kline.Symbol,
				Side:             types.SideTypeSell,
				Type:             types.OrderTypeLimit,
				Price:            sellPrice,
				Quantity:         quantity,
				MarginSideEffect: types.SideEffectTypeMarginBuy,
				Tag:              "FailedBreakHighLimit",
			})
		}
	}))
}

func (s *FailedBreakHigh) pilotQuantityCalculation() {
	log.Infof("pilot calculation for max position: last low = %f, quantity = %f, leverage = %f",
		s.lastHigh.Float64(),
		s.Quantity.Float64(),
		s.Leverage.Float64())

	quantity, err := risk.CalculateBaseQuantity(s.session, s.Market, s.lastHigh, s.Quantity, s.Leverage)
	if err != nil {
		log.WithError(err).Errorf("quantity calculation error")
	}

	if quantity.IsZero() {
		log.WithError(err).Errorf("quantity is zero, can not submit order")
		return
	}

	bbgo.Notify("%s %f quantity will be used for failed break high short", s.Symbol, quantity.Float64())
}

func (s *FailedBreakHigh) updatePivotHigh() bool {
	lastHigh := fixedpoint.NewFromFloat(s.pivotHigh.Last())
	if lastHigh.IsZero() || lastHigh.Compare(s.lastHigh) == 0 {
		return false
	}

	s.lastHigh = lastHigh
	s.PivotHighPrices = append(s.PivotHighPrices, lastHigh)
	return true
}
