package pivotshort

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type FakeBreakStop struct {
	types.IntervalWindow
}

// BreakLow -- when price breaks the previous pivot low, we set a trade entry
type BreakLow struct {
	Symbol string
	Market types.Market
	types.IntervalWindow

	// FastWindow is used for fast pivot (this is to filter the nearest high/low)
	FastWindow int `json:"fastWindow"`

	// Ratio is a number less than 1.0, price * ratio will be the price triggers the short order.
	Ratio fixedpoint.Value `json:"ratio"`

	bbgo.OpenPositionOptions

	// BounceRatio is a ratio used for placing the limit order sell price
	// limit sell price = breakLowPrice * (1 + BounceRatio)
	BounceRatio fixedpoint.Value `json:"bounceRatio"`

	StopEMA *bbgo.StopEMA `json:"stopEMA"`

	TrendEMA *bbgo.TrendEMA `json:"trendEMA"`

	FakeBreakStop *FakeBreakStop `json:"fakeBreakStop"`

	lastLow, lastFastLow fixedpoint.Value

	lastLowInvalidated bool

	// lastBreakLow is the low that the price just break
	lastBreakLow fixedpoint.Value

	pivotLow, fastPivotLow *indicator.PivotLow
	pivotLowPrices         []fixedpoint.Value

	orderExecutor *bbgo.GeneralOrderExecutor
	session       *bbgo.ExchangeSession

	// StrategyController
	bbgo.StrategyController
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

	if s.FakeBreakStop != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.FakeBreakStop.Interval})
	}
}

func (s *BreakLow) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	if s.FastWindow == 0 {
		s.FastWindow = 3
	}

	s.session = session
	s.orderExecutor = orderExecutor

	// StrategyController
	s.Status = types.StrategyStatusRunning

	position := orderExecutor.Position()
	symbol := position.Symbol
	standardIndicator := session.StandardIndicatorSet(s.Symbol)

	s.lastLow = fixedpoint.Zero
	s.pivotLow = standardIndicator.PivotLow(s.IntervalWindow)
	s.fastPivotLow = standardIndicator.PivotLow(types.IntervalWindow{
		Interval: s.Interval,
		Window:   s.FastWindow, // make it faster
	})

	if s.StopEMA != nil {
		s.StopEMA.Bind(session, orderExecutor)
	}

	if s.TrendEMA != nil {
		s.TrendEMA.Bind(session, orderExecutor)
	}

	// update pivot low data
	session.MarketDataStream.OnStart(func() {
		if s.updatePivotLow() {
			bbgo.Notify("%s new pivot low: %f", s.Symbol, s.pivotLow.Last(0))
		}

		s.pilotQuantityCalculation()
	})

	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {
		if s.updatePivotLow() {
			// when position is opened, do not send pivot low notify
			if position.IsOpened(kline.Close) {
				return
			}

			bbgo.Notify("%s new pivot low: %f", s.Symbol, s.pivotLow.Last(0))
		}
	}))

	if s.FakeBreakStop != nil {
		// if the position is already opened, and we just break the low, this checks if the kline closed above the low,
		// so that we can close the position earlier
		session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.FakeBreakStop.Interval, func(k types.KLine) {
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
				if err := s.orderExecutor.ClosePosition(context.Background(), one, "fakeBreakStop"); err != nil {
					log.WithError(err).Error("position close error")
				}

				// reset to zero
				s.lastBreakLow = fixedpoint.Zero
			}
		}))
	}

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(kline types.KLine) {
		if len(s.pivotLowPrices) == 0 || s.lastLow.IsZero() {
			log.Infof("currently there is no pivot low prices, can not check break low...")
			return
		}

		if s.lastLowInvalidated {
			log.Infof("the last low is invalidated, skip")
			return
		}

		previousLow := s.lastLow
		ratio := fixedpoint.One.Add(s.Ratio)
		breakPrice := previousLow.Mul(ratio)

		// StrategyController
		if s.Status != types.StrategyStatusRunning {
			return
		}

		openPrice := kline.Open
		closePrice := kline.Close

		// if the previous low is not break, or the kline is not strong enough to break it, skip
		if closePrice.Compare(breakPrice) >= 0 {
			return
		}

		// we need the price cross the break line, or we do nothing:
		// 1) open > break price > close price
		// 2) high > break price > open price and close price
		// v2
		if !((openPrice.Compare(breakPrice) > 0 && closePrice.Compare(breakPrice) < 0) ||
			(kline.High.Compare(breakPrice) > 0 && openPrice.Compare(breakPrice) < 0 && closePrice.Compare(breakPrice) < 0)) {
			return
		}

		// force direction to be down
		if closePrice.Compare(openPrice) >= 0 {
			bbgo.Notify("%s price %f is closed higher than the open price %f, skip this break", kline.Symbol, closePrice.Float64(), openPrice.Float64())
			// skip UP klines
			return
		}

		bbgo.Notify("%s breakLow signal detected, closed price %f < breakPrice %f", kline.Symbol, closePrice.Float64(), breakPrice.Float64(), kline)

		if s.lastBreakLow.IsZero() || previousLow.Compare(s.lastBreakLow) < 0 {
			s.lastBreakLow = previousLow
		}

		if position.IsOpened(kline.Close) {
			bbgo.Notify("%s position is already opened, skip", s.Symbol)
			return
		}

		// trend EMA protection
		if s.TrendEMA != nil && !s.TrendEMA.GradientAllowed() {
			bbgo.Notify("trendEMA protection: %s close price %f, gradient %f", s.Symbol, kline.Close.Float64(), s.TrendEMA.Gradient())
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

		bbgo.Notify("%s price %f breaks the previous low %f with ratio %f, opening short position", symbol, kline.Close.Float64(), previousLow.Float64(), s.Ratio.Float64())
		opts := s.OpenPositionOptions
		opts.Short = true
		opts.Price = closePrice
		opts.Tags = []string{"breakLowMarket"}
		if opts.LimitOrder && !s.BounceRatio.IsZero() {
			opts.Price = previousLow.Mul(fixedpoint.One.Add(s.BounceRatio))
		}

		if _, err := s.orderExecutor.OpenPosition(ctx, opts); err != nil {
			log.WithError(err).Errorf("failed to open short position")
		}
	}))
}

func (s *BreakLow) pilotQuantityCalculation() {
	if s.lastLow.IsZero() {
		return
	}

	log.Infof("pilot calculation for max position: last low = %f, quantity = %f, leverage = %f",
		s.lastLow.Float64(),
		s.Quantity.Float64(),
		s.Leverage.Float64())

	quantity, err := bbgo.CalculateBaseQuantity(s.session, s.Market, s.lastLow, s.Quantity, s.Leverage)
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
	low := fixedpoint.NewFromFloat(s.pivotLow.Last(0))
	if low.IsZero() {
		return false
	}

	// if the last low is different
	lastLowChanged := low.Compare(s.lastLow) != 0
	if lastLowChanged {
		s.lastLow = low
		s.lastLowInvalidated = false
		s.pivotLowPrices = append(s.pivotLowPrices, low)
	}

	fastLow := fixedpoint.NewFromFloat(s.fastPivotLow.Last(0))
	if !fastLow.IsZero() {
		if fastLow.Compare(s.lastLow) < 0 {
			s.lastLowInvalidated = true
			lastLowChanged = false
		}
		s.lastFastLow = fastLow
	}

	return lastLowChanged
}
