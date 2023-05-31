package trendtrader

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type TrendLine struct {
	Symbol string
	Market types.Market `json:"-"`
	types.IntervalWindow

	PivotRightWindow fixedpoint.Value `json:"pivotRightWindow"`

	// MarketOrder is the option to enable market order short.
	MarketOrder bool `json:"marketOrder"`

	Quantity fixedpoint.Value `json:"quantity"`

	orderExecutor *bbgo.GeneralOrderExecutor
	session       *bbgo.ExchangeSession
	activeOrders  *bbgo.ActiveOrderBook

	pivotHigh *indicator.PivotHigh
	pivotLow  *indicator.PivotLow

	bbgo.QuantityOrAmount
}

func (s *TrendLine) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})

	//if s.pivot != nil {
	//	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	//}
}

func (s *TrendLine) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor

	position := orderExecutor.Position()
	symbol := position.Symbol
	standardIndicator := session.StandardIndicatorSet(s.Symbol)
	s.pivotHigh = standardIndicator.PivotHigh(types.IntervalWindow{s.Interval, int(3. * s.PivotRightWindow.Float64()), int(s.PivotRightWindow.Float64())})
	s.pivotLow = standardIndicator.PivotLow(types.IntervalWindow{s.Interval, int(3. * s.PivotRightWindow.Float64()), int(s.PivotRightWindow.Float64())})

	resistancePrices := types.NewQueue(3)
	pivotHighDurationCounter := 0.
	resistanceDuration := types.NewQueue(2)
	supportPrices := types.NewQueue(3)
	pivotLowDurationCounter := 0.
	supportDuration := types.NewQueue(2)

	resistanceSlope := 0.
	resistanceSlope1 := 0.
	resistanceSlope2 := 0.
	supportSlope := 0.
	supportSlope1 := 0.
	supportSlope2 := 0.

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		if s.pivotHigh.Last(0) != resistancePrices.Last(0) {
			resistancePrices.Update(s.pivotHigh.Last(0))
			resistanceDuration.Update(pivotHighDurationCounter)
			pivotHighDurationCounter = 0
		} else {
			pivotHighDurationCounter++
		}
		if s.pivotLow.Last(0) != supportPrices.Last(0) {
			supportPrices.Update(s.pivotLow.Last(0))
			supportDuration.Update(pivotLowDurationCounter)
			pivotLowDurationCounter = 0
		} else {
			pivotLowDurationCounter++
		}

		if line(resistancePrices.Index(2), resistancePrices.Index(1), resistancePrices.Index(0)) < 0 {
			resistanceSlope1 = (resistancePrices.Index(1) - resistancePrices.Index(2)) / resistanceDuration.Index(1)
			resistanceSlope2 = (resistancePrices.Index(0) - resistancePrices.Index(1)) / resistanceDuration.Index(0)

			resistanceSlope = (resistanceSlope1 + resistanceSlope2) / 2.
		}
		if line(supportPrices.Index(2), supportPrices.Index(1), supportPrices.Index(0)) > 0 {
			supportSlope1 = (supportPrices.Index(1) - supportPrices.Index(2)) / supportDuration.Index(1)
			supportSlope2 = (supportPrices.Index(0) - supportPrices.Index(1)) / supportDuration.Index(0)

			supportSlope = (supportSlope1 + supportSlope2) / 2.
		}

		if converge(resistanceSlope, supportSlope) {
			// y = mx+b
			currentResistance := resistanceSlope*pivotHighDurationCounter + resistancePrices.Last(0)
			currentSupport := supportSlope*pivotLowDurationCounter + supportPrices.Last(0)
			log.Info(currentResistance, currentSupport, kline.Close)

			if kline.High.Float64() > currentResistance {
				if position.IsShort() {
					s.orderExecutor.ClosePosition(context.Background(), one)
				}
				if position.IsDust(kline.Close) || position.IsClosed() {
					s.placeOrder(context.Background(), types.SideTypeBuy, s.Quantity, symbol) // OrAmount.CalculateQuantity(kline.Close)
				}

			} else if kline.Low.Float64() < currentSupport {
				if position.IsLong() {
					s.orderExecutor.ClosePosition(context.Background(), one)
				}
				if position.IsDust(kline.Close) || position.IsClosed() {
					s.placeOrder(context.Background(), types.SideTypeSell, s.Quantity, symbol) // OrAmount.CalculateQuantity(kline.Close)
				}
			}
		}
	}))

	if !bbgo.IsBackTesting {
		session.MarketDataStream.OnMarketTrade(func(trade types.Trade) {
		})
	}
}

func (s *TrendLine) placeOrder(ctx context.Context, side types.SideType, quantity fixedpoint.Value, symbol string) error {
	market, _ := s.session.Market(symbol)
	_, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:   symbol,
		Market:   market,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: quantity,
		Tag:      "trend-break",
	})
	if err != nil {
		log.WithError(err).Errorf("can not place market order")
	}
	return err
}

func line(p1, p2, p3 float64) int64 {
	if p1 >= p2 && p2 >= p3 {
		return -1
	} else if p1 <= p2 && p2 <= p3 {
		return +1
	}
	return 0
}

func converge(mr, ms float64) bool {
	return ms > mr
}
