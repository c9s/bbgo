package pivotshort

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// FailedBreakHigh -- when price breaks the previous pivot low, we set a trade entry
type FailedBreakHigh struct {
	Symbol string
	Market types.Market

	// IntervalWindow is used for finding the pivot high
	types.IntervalWindow

	bbgo.OpenPositionOptions

	// BreakInterval is used for checking failed break
	BreakInterval types.Interval `json:"breakInterval"`

	Enabled bool `json:"enabled"`

	// Ratio is a number less than 1.0, price * ratio will be the price triggers the short order.
	Ratio fixedpoint.Value `json:"ratio"`

	VWMA *types.IntervalWindow `json:"vwma"`

	StopEMA *bbgo.StopEMA `json:"stopEMA"`

	TrendEMA *bbgo.TrendEMA `json:"trendEMA"`

	lastFailedBreakHigh, lastHigh, lastFastHigh fixedpoint.Value

	pivotHigh, fastPivotHigh *indicator.PivotHigh
	vwma                     *indicator.VWMA
	pivotHighPrices          []fixedpoint.Value

	orderExecutor *bbgo.GeneralOrderExecutor
	session       *bbgo.ExchangeSession

	// StrategyController
	bbgo.StrategyController
}

func (s *FailedBreakHigh) Subscribe(session *bbgo.ExchangeSession) {
	if s.BreakInterval == "" {
		s.BreakInterval = types.Interval1m
	}

	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.BreakInterval})

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
	s.fastPivotHigh = standardIndicator.PivotHigh(types.IntervalWindow{
		Interval: s.IntervalWindow.Interval,
		Window:   3,
	})

	// StrategyController
	s.Status = types.StrategyStatusRunning

	if s.VWMA != nil {
		s.vwma = standardIndicator.VWMA(types.IntervalWindow{
			Interval: s.BreakInterval,
			Window:   s.VWMA.Window,
		})
	}

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
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.BreakInterval, func(k types.KLine) {
		if !s.Enabled {
			return
		}

		// StrategyController
		if s.Status != types.StrategyStatusRunning {
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
			bbgo.Notify("kLine closed above the last break high, triggering stop earlier")
			if err := s.orderExecutor.ClosePosition(context.Background(), one, "failedBreakHighStop"); err != nil {
				log.WithError(err).Error("position close error")
			}

			// reset to zero
			s.lastFailedBreakHigh = fixedpoint.Zero
		}
	}))

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.BreakInterval, func(kline types.KLine) {
		if len(s.pivotHighPrices) == 0 || s.lastHigh.IsZero() {
			log.Infof("currently there is no pivot high prices, can not check failed break high...")
			return
		}

		// StrategyController
		if s.Status != types.StrategyStatusRunning {
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

		if s.vwma != nil {
			vma := fixedpoint.NewFromFloat(s.vwma.Last())
			if kline.Volume.Compare(vma) < 0 {
				bbgo.Notify("%s %s kline volume %f is less than VMA %f, skip failed break high short", kline.Symbol, kline.Interval, kline.Volume.Float64(), vma.Float64())
				return
			}
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
				bbgo.Notify("stopEMA protection: close price %f %s", kline.Close.Float64(), s.StopEMA.String())
				return
			}
		}

		ctx := context.Background()

		bbgo.Notify("%s price %f failed breaking the previous high %f with ratio %f, opening short position",
			symbol,
			kline.Close.Float64(),
			previousHigh.Float64(),
			s.Ratio.Float64())

		// graceful cancel all active orders
		_ = orderExecutor.GracefulCancel(ctx)

		opts := s.OpenPositionOptions
		opts.Short = true
		opts.Price = closePrice
		opts.Tags = []string{"FailedBreakHighMarket"}
		if err := s.orderExecutor.OpenPosition(ctx, opts); err != nil {
			log.WithError(err).Errorf("failed to open short position")
		}
	}))
}

func (s *FailedBreakHigh) pilotQuantityCalculation() {
	log.Infof("pilot calculation for max position: last low = %f, quantity = %f, leverage = %f",
		s.lastHigh.Float64(),
		s.Quantity.Float64(),
		s.Leverage.Float64())

	quantity, err := bbgo.CalculateBaseQuantity(s.session, s.Market, s.lastHigh, s.Quantity, s.Leverage)
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
	if lastHigh.IsZero() {
		return false
	}

	lastHighChanged := lastHigh.Compare(s.lastHigh) != 0
	if lastHighChanged {
		s.lastHigh = lastHigh
		s.pivotHighPrices = append(s.pivotHighPrices, lastHigh)
	}

	lastFastHigh := fixedpoint.NewFromFloat(s.fastPivotHigh.Last())
	if !lastFastHigh.IsZero() {
		if lastFastHigh.Compare(s.lastHigh) > 0 {
			// invalidate the last low
			s.lastHigh = fixedpoint.Zero
			lastHighChanged = false
		}
		s.lastFastHigh = lastFastHigh
	}

	return lastHighChanged
}
