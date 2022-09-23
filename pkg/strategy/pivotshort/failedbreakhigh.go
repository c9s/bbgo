package pivotshort

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type MACDDivergence struct {
	*indicator.MACDConfig
	PivotWindow int `json:"pivotWindow"`
}

// FailedBreakHigh -- when price breaks the previous pivot low, we set a trade entry
type FailedBreakHigh struct {
	Symbol string
	Market types.Market

	// IntervalWindow is used for finding the pivot high
	types.IntervalWindow

	FastWindow int

	bbgo.OpenPositionOptions

	// BreakInterval is used for checking failed break
	BreakInterval types.Interval `json:"breakInterval"`

	Enabled bool `json:"enabled"`

	// Ratio is a number less than 1.0, price * ratio will be the price triggers the short order.
	Ratio fixedpoint.Value `json:"ratio"`

	// EarlyStopRatio adjusts the break high price with the given ratio
	// this is for stop loss earlier if the price goes above the previous price
	EarlyStopRatio fixedpoint.Value `json:"earlyStopRatio"`

	VWMA *types.IntervalWindow `json:"vwma"`

	StopEMA *bbgo.StopEMA `json:"stopEMA"`

	TrendEMA *bbgo.TrendEMA `json:"trendEMA"`

	MACDDivergence *MACDDivergence `json:"macdDivergence"`

	macd *indicator.MACD

	macdTopDivergence bool

	lastFailedBreakHigh, lastHigh, lastFastHigh fixedpoint.Value
	lastHighInvalidated                         bool
	pivotHighPrices                             []fixedpoint.Value

	pivotHigh, fastPivotHigh *indicator.PivotHigh
	vwma                     *indicator.VWMA

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

	if s.MACDDivergence != nil {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.MACDDivergence.Interval})
	}
}

func (s *FailedBreakHigh) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	if !s.Enabled {
		return
	}

	// set default value for StrategyController
	s.Status = types.StrategyStatusRunning

	if s.FastWindow == 0 {
		s.FastWindow = 3
	}

	position := orderExecutor.Position()
	symbol := position.Symbol
	standardIndicator := session.StandardIndicatorSet(s.Symbol)

	s.lastHigh = fixedpoint.Zero
	s.pivotHigh = standardIndicator.PivotHigh(s.IntervalWindow)
	s.fastPivotHigh = standardIndicator.PivotHigh(types.IntervalWindow{
		Interval: s.IntervalWindow.Interval,
		Window:   s.FastWindow,
	})

	// Experimental: MACD divergence detection
	if s.MACDDivergence != nil {
		log.Infof("MACD divergence detection is enabled")
		s.macd = standardIndicator.MACD(s.MACDDivergence.IntervalWindow, s.MACDDivergence.ShortPeriod, s.MACDDivergence.LongPeriod)
		s.macd.OnUpdate(func(macd, signal, histogram float64) {
			log.Infof("MACD %+v: macd: %f, signal: %f histogram: %f", s.macd.IntervalWindow, macd, signal, histogram)
			s.detectMacdDivergence()
		})
		s.detectMacdDivergence()
	}

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

			bbgo.Notify("%s new pivot high: %f", s.Symbol, s.pivotHigh.Last())
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

		lastHigh := s.lastFastHigh

		if !s.EarlyStopRatio.IsZero() {
			lastHigh = lastHigh.Mul(one.Add(s.EarlyStopRatio))
		}

		// the kline opened below the last break low, and closed above the last break low
		if k.Open.Compare(lastHigh) < 0 && k.Close.Compare(lastHigh) > 0 && k.Open.Compare(k.Close) > 0 {
			bbgo.Notify("kLine closed %f above the last break high %f (ratio %f), triggering stop earlier", k.Close.Float64(), lastHigh.Float64(), s.EarlyStopRatio.Float64())

			if err := s.orderExecutor.ClosePosition(context.Background(), one, "failedBreakHighStop"); err != nil {
				log.WithError(err).Error("position close error")
			}

			// reset to zero
			s.lastFailedBreakHigh = fixedpoint.Zero
		}
	}))

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.BreakInterval, func(kline types.KLine) {
		if len(s.pivotHighPrices) == 0 || s.lastHigh.IsZero() {
			log.Infof("%s currently there is no pivot high prices, can not check failed break high...", s.Symbol)
			return
		}

		if s.lastHighInvalidated {
			log.Infof("%s last high %f is invalidated by the fast pivot", s.Symbol, s.lastHigh.Float64())
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
		if kline.High.Compare(breakPrice) < 0 || closePrice.Compare(breakPrice) >= 0 {
			return
		}

		// 3) kline.Close is lower than kline.Open
		if closePrice.Compare(openPrice) > 0 {
			bbgo.Notify("the %s closed price %f is higher than the open price %f, skip failed break high short", s.Symbol, closePrice.Float64(), openPrice.Float64())
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

		if position.IsOpened(kline.Close) {
			bbgo.Notify("%s position is already opened, skip", s.Symbol)
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

		if s.macd != nil && !s.macdTopDivergence {
			bbgo.Notify("Detected MACD top divergence")
			return
		}

		if s.lastFailedBreakHigh.IsZero() || previousHigh.Compare(s.lastFailedBreakHigh) < 0 {
			s.lastFailedBreakHigh = previousHigh
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
		if _, err := s.orderExecutor.OpenPosition(ctx, opts); err != nil {
			log.WithError(err).Errorf("failed to open short position")
		}
	}))
}

func (s *FailedBreakHigh) pilotQuantityCalculation() {
	if s.lastHigh.IsZero() {
		return
	}

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

func (s *FailedBreakHigh) detectMacdDivergence() {
	if s.MACDDivergence == nil {
		return
	}

	// always reset the top divergence to false
	s.macdTopDivergence = false

	histogramValues := s.macd.Histogram

	pivotWindow := s.MACDDivergence.PivotWindow
	if pivotWindow == 0 {
		pivotWindow = 3
	}

	if len(histogramValues) < pivotWindow*2 {
		log.Warnf("histogram values is not enough for finding pivots, length=%d", len(histogramValues))
		return
	}

	var histogramPivots floats.Slice
	for i := pivotWindow; i > 0 && i < len(histogramValues); i++ {
		// find positive histogram and the top
		pivot, ok := floats.CalculatePivot(histogramValues[0:i], pivotWindow, pivotWindow, func(a, pivot float64) bool {
			return pivot > 0 && pivot > a
		})
		if ok {
			histogramPivots = append(histogramPivots, pivot)
		}
	}
	log.Infof("histogram pivots: %+v", histogramPivots)

	// take the last 2-3 pivots to check if there is a divergence
	if len(histogramPivots) < 3 {
		return
	}

	histogramPivots = histogramPivots[len(histogramPivots)-3:]
	minDiff := 0.01
	for i := len(histogramPivots) - 1; i > 0; i-- {
		p1 := histogramPivots[i]
		p2 := histogramPivots[i-1]
		diff := p1 - p2

		if diff > -minDiff || diff > minDiff {
			continue
		}

		// negative value = MACD top divergence
		if diff < -minDiff {
			log.Infof("MACD TOP DIVERGENCE DETECTED: diff %f", diff)
			s.macdTopDivergence = true
		} else {
			s.macdTopDivergence = false
		}
		return
	}
}

func (s *FailedBreakHigh) updatePivotHigh() bool {
	high := fixedpoint.NewFromFloat(s.pivotHigh.Last())
	if high.IsZero() {
		return false
	}

	lastHighChanged := high.Compare(s.lastHigh) != 0
	if lastHighChanged {
		s.lastHigh = high
		s.lastHighInvalidated = false
		s.pivotHighPrices = append(s.pivotHighPrices, high)
	}

	fastHigh := fixedpoint.NewFromFloat(s.fastPivotHigh.Last())
	if !fastHigh.IsZero() {
		if fastHigh.Compare(s.lastHigh) > 0 {
			// invalidate the last low
			lastHighChanged = false
			s.lastHighInvalidated = true
		}
		s.lastFastHigh = fastHigh
	}

	return lastHighChanged
}
