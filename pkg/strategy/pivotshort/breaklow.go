package pivotshort

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
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

	lastLow        fixedpoint.Value
	pivot          *indicator.Pivot
	stopEWMA       *indicator.EWMA
	pivotLowPrices []fixedpoint.Value

	orderExecutor *bbgo.GeneralOrderExecutor
	session       *bbgo.ExchangeSession
}

func (s *BreakLow) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	symbol := position.Symbol
	store, _ := session.MarketDataStore(symbol)
	standardIndicator, _ := session.StandardIndicatorSet(symbol)

	s.lastLow = fixedpoint.Zero

	s.pivot = &indicator.Pivot{IntervalWindow: s.IntervalWindow}
	s.pivot.Bind(store)
	preloadPivot(s.pivot, store)

	if s.StopEMA != nil {
		s.stopEWMA = standardIndicator.EWMA(*s.StopEMA)
	}

	// update pivot low data
	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, s.Interval, func(kline types.KLine) {
		lastLow := fixedpoint.NewFromFloat(s.pivot.LastLow())
		if lastLow.IsZero() {
			return
		}

		if lastLow.Compare(s.lastLow) != 0 {
			log.Infof("new pivot low detected: %f %s", s.pivot.LastLow(), kline.EndTime.Time())
		}

		s.lastLow = lastLow
		s.pivotLowPrices = append(s.pivotLowPrices, s.lastLow)
	}))

	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, types.Interval1m, func(kline types.KLine) {
		if position.IsOpened(kline.Close) {
			return
		}

		if len(s.pivotLowPrices) == 0 {
			log.Infof("currently there is no pivot low prices, skip placing orders...")
			return
		}

		previousLow := s.pivotLowPrices[len(s.pivotLowPrices)-1]

		// truncate the pivot low prices
		if len(s.pivotLowPrices) > 10 {
			s.pivotLowPrices = s.pivotLowPrices[len(s.pivotLowPrices)-10:]
		}

		ratio := fixedpoint.One.Add(s.Ratio)
		breakPrice := previousLow.Mul(ratio)

		openPrice := kline.Open
		closePrice := kline.Close
		// if previous low is not break, skip
		if closePrice.Compare(breakPrice) >= 0 {
			return
		}

		// we need the price cross the break line or we do nothing
		if !(openPrice.Compare(breakPrice) > 0 && closePrice.Compare(breakPrice) < 0) {
			return
		}

		log.Infof("%s breakLow signal detected, closed price %f < breakPrice %f", kline.Symbol, closePrice.Float64(), breakPrice.Float64())

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

		quantity := s.useQuantityOrBaseBalance(s.Quantity)
		if s.MarketOrder {
			bbgo.Notify("%s price %f breaks the previous low %f with ratio %f, submitting market sell to open a short position", symbol, kline.Close.Float64(), previousLow.Float64(), s.Ratio.Float64())
			s.placeMarketSell(ctx, quantity, "breakLowMarket")
		} else {
			sellPrice := previousLow.Mul(fixedpoint.One.Add(s.BounceRatio))

			bbgo.Notify("%s price %f breaks the previous low %f with ratio %f, submitting limit sell @ %f", symbol, kline.Close.Float64(), previousLow.Float64(), s.Ratio.Float64(), sellPrice.Float64())
			s.placeLimitSell(ctx, sellPrice, quantity, "breakLowLimit")
		}
	}))

	if !bbgo.IsBackTesting {
		// use market trade to submit short order
		session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {

		})
	}
}

func (s *BreakLow) useQuantityOrBaseBalance(quantity fixedpoint.Value) fixedpoint.Value {
	if s.session.Margin || s.session.IsolatedMargin || s.session.Futures || s.session.IsolatedFutures {
		return quantity
	}

	balance, hasBalance := s.session.Account.Balance(s.Market.BaseCurrency)
	if hasBalance {
		if quantity.IsZero() {
			bbgo.Notify("sell quantity is not set, submitting sell with all base balance: %s", balance.Available.String())
			quantity = balance.Available
		} else {
			quantity = fixedpoint.Min(quantity, balance.Available)
		}
	}

	if quantity.IsZero() {
		log.Errorf("quantity is zero, can not submit sell order, please check settings")
	}

	return quantity
}

func (s *BreakLow) placeLimitSell(ctx context.Context, price, quantity fixedpoint.Value, tag string) {
	_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:           s.Symbol,
		Price:            price,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeLimit,
		Quantity:         quantity,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
		Tag:              tag,
	})
}

func (s *BreakLow) placeMarketSell(ctx context.Context, quantity fixedpoint.Value, tag string) {
	_, _ = s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:           s.Symbol,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
		Tag:              tag,
	})
}
