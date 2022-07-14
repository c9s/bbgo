package pivotshort

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/risk"
	"github.com/c9s/bbgo/pkg/types"
)

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

	Quantity     fixedpoint.Value      `json:"quantity"`
	StopEMARange fixedpoint.Value      `json:"stopEMARange"`
	StopEMA      *types.IntervalWindow `json:"stopEMA"`

	TrendEMA *types.IntervalWindow `json:"trendEMA"`

	lastLow  fixedpoint.Value
	pivot    *indicator.Pivot
	stopEWMA *indicator.EWMA

	trendEWMA                       *indicator.EWMA
	trendEWMALast, trendEWMACurrent float64

	pivotLowPrices []fixedpoint.Value

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
}

func (s *BreakLow) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	symbol := position.Symbol
	store, _ := session.MarketDataStore(s.Symbol)
	standardIndicator, _ := session.StandardIndicatorSet(s.Symbol)

	s.lastLow = fixedpoint.Zero

	s.pivot = &indicator.Pivot{IntervalWindow: s.IntervalWindow}
	s.pivot.Bind(store)
	preloadPivot(s.pivot, store)

	if s.StopEMA != nil {
		s.stopEWMA = standardIndicator.EWMA(*s.StopEMA)
	}

	if s.TrendEMA != nil {
		s.trendEWMA = standardIndicator.EWMA(*s.TrendEMA)

		session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.TrendEMA.Interval, func(kline types.KLine) {
			s.trendEWMALast = s.trendEWMACurrent
			s.trendEWMACurrent = s.trendEWMA.Last()
		}))
	}

	// update pivot low data
	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {
		lastLow := fixedpoint.NewFromFloat(s.pivot.LastLow())
		if lastLow.IsZero() {
			return
		}

		if lastLow.Compare(s.lastLow) != 0 {
			bbgo.Notify("%s new pivot low detected: %f %s", s.Symbol, s.pivot.LastLow(), kline.EndTime.Time().String())
		}

		s.lastLow = lastLow
		s.pivotLowPrices = append(s.pivotLowPrices, s.lastLow)
	}))

	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, types.Interval1m, func(kline types.KLine) {
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

			emaStopShortPrice := ema.Mul(fixedpoint.One.Sub(s.StopEMARange))
			if closePrice.Compare(emaStopShortPrice) < 0 {
				log.Infof("stopEMA protection: close price %f < EMA(%v) = %f", closePrice.Float64(), s.StopEMA, ema.Float64())
				return
			}
		}

		ctx := context.Background()

		// graceful cancel all active orders
		_ = orderExecutor.GracefulCancel(ctx)

		leverage := fixedpoint.NewFromInt(5)
		quantity, err := useQuantityOrBaseBalance(s.session, s.Market, closePrice, s.Quantity, leverage)
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

	if !bbgo.IsBackTesting {
		// use market trade to submit short order
		session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {

		})
	}
}

func useQuantityOrBaseBalance(session *bbgo.ExchangeSession, market types.Market, price, quantity, leverage fixedpoint.Value) (fixedpoint.Value, error) {
	usingLeverage := session.Margin || session.IsolatedMargin || session.Futures || session.IsolatedFutures
	if usingLeverage {
		if !quantity.IsZero() {
			return quantity, nil
		}

		// quantity is zero, we need to calculate the quantity
		baseBalance, _ := session.Account.Balance(market.BaseCurrency)
		quoteBalance, _ := session.Account.Balance(market.QuoteCurrency)

		// calculate the quantity automatically
		if session.Margin || session.IsolatedMargin {
			baseBalanceValue := baseBalance.Total().Mul(price)
			accountValue := baseBalanceValue.Add(quoteBalance.Total())

			if session.IsolatedMargin {
				originLeverage := leverage
				leverage = fixedpoint.Max(leverage, fixedpoint.NewFromInt(10))
				log.Infof("using isolated margin, maxLeverage=10 originalLeverage=%f currentLeverage=%f",
					originLeverage.Float64(),
					leverage.Float64())
			}

			// spot margin use the equity value, so we use the total quote balance here
			maxPositionQuantity := risk.CalculateMaxPosition(price, accountValue, leverage)

			log.Infof("margin leverage: calculated maxPositionQuantity=%f price=%f accountValue=%f %s leverage=%f",
				maxPositionQuantity.Float64(),
				price.Float64(),
				accountValue.Float64(),
				market.QuoteCurrency,
				leverage.Float64())

			return maxPositionQuantity, nil
		}

		if session.Futures || session.IsolatedFutures {
			// TODO: get mark price here
			maxPositionQuantity := risk.CalculateMaxPosition(price, quoteBalance.Available, leverage)
			requiredPositionCost := risk.CalculatePositionCost(price, price, maxPositionQuantity, leverage, types.SideTypeSell)
			if quoteBalance.Available.Compare(requiredPositionCost) < 0 {
				return maxPositionQuantity, fmt.Errorf("available margin %f %s is not enough, can not submit order", quoteBalance.Available.Float64(), market.QuoteCurrency)
			}

			return maxPositionQuantity, nil
		}

	}

	// For spot, we simply sell the base currency
	balance, hasBalance := session.Account.Balance(market.BaseCurrency)
	if hasBalance {
		if quantity.IsZero() {
			log.Warnf("sell quantity is not set, submitting sell with all base balance: %s", balance.Available.String())
			quantity = balance.Available
		} else {
			quantity = fixedpoint.Min(quantity, balance.Available)
		}
	}

	return quantity, fmt.Errorf("quantity is zero, can not submit sell order, please check your settings")
}
