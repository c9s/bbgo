package bbgo

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// SupportTakeProfit finds the previous support price and take profit at the previous low.
type SupportTakeProfit struct {
	Symbol string
	types.IntervalWindow

	Ratio fixedpoint.Value `json:"ratio"`

	pivot               *indicator.PivotLow
	orderExecutor       *GeneralOrderExecutor
	session             *ExchangeSession
	activeOrders        *ActiveOrderBook
	currentSupportPrice fixedpoint.Value

	triggeredPrices []fixedpoint.Value
}

func (s *SupportTakeProfit) Subscribe(session *ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *SupportTakeProfit) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	s.session = session
	s.orderExecutor = orderExecutor
	s.activeOrders = NewActiveOrderBook(s.Symbol)
	session.UserDataStream.OnOrderUpdate(func(order types.Order) {
		if s.activeOrders.Exists(order) {
			if !s.currentSupportPrice.IsZero() {
				s.triggeredPrices = append(s.triggeredPrices, s.currentSupportPrice)
			}
		}
	})
	s.activeOrders.BindStream(session.UserDataStream)

	position := orderExecutor.Position()

	s.pivot = session.StandardIndicatorSet(s.Symbol).PivotLow(s.IntervalWindow)

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(kline types.KLine) {
		if !s.updateSupportPrice(kline.Close) {
			return
		}

		if !position.IsOpened(kline.Close) {
			logrus.Infof("position is not opened, skip updating support take profit order")
			return
		}

		buyPrice := s.currentSupportPrice.Mul(one.Add(s.Ratio))
		quantity := position.GetQuantity()
		ctx := context.Background()

		if err := orderExecutor.GracefulCancelActiveOrderBook(ctx, s.activeOrders); err != nil {
			logrus.WithError(err).Errorf("cancel order failed")
		}

		Notify("placing %s take profit order at price %f", s.Symbol, buyPrice.Float64())
		createdOrders, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:           s.Symbol,
			Type:             types.OrderTypeLimitMaker,
			Side:             types.SideTypeBuy,
			Price:            buyPrice,
			Quantity:         quantity,
			Tag:              "supportTakeProfit",
			MarginSideEffect: types.SideEffectTypeAutoRepay,
		})

		if err != nil {
			logrus.WithError(err).Errorf("can not submit orders: %+v", createdOrders)
		}

		s.activeOrders.Add(createdOrders...)
	}))
}

func (s *SupportTakeProfit) updateSupportPrice(closePrice fixedpoint.Value) bool {
	logrus.Infof("[supportTakeProfit] lows: %v", s.pivot.Values)

	groupDistance := 0.01
	minDistance := 0.05
	supportPrices := findPossibleSupportPrices(closePrice.Float64()*(1.0-minDistance), groupDistance, s.pivot.Values)
	if len(supportPrices) == 0 {
		return false
	}

	logrus.Infof("[supportTakeProfit] found possible support prices: %v", supportPrices)

	// nextSupportPrice are sorted in increasing order
	nextSupportPrice := fixedpoint.NewFromFloat(supportPrices[len(supportPrices)-1])

	// it's price that we have been used to take profit
	for _, p := range s.triggeredPrices {
		var l = p.Mul(one.Sub(fixedpoint.NewFromFloat(0.01)))
		var h = p.Mul(one.Add(fixedpoint.NewFromFloat(0.01)))
		if p.Compare(l) > 0 && p.Compare(h) < 0 {
			return false
		}
	}

	currentBuyPrice := s.currentSupportPrice.Mul(one.Add(s.Ratio))

	if s.currentSupportPrice.IsZero() {
		logrus.Infof("setup next support take profit price at %f", nextSupportPrice.Float64())
		s.currentSupportPrice = nextSupportPrice
		return true
	}

	// the close price is already lower than the support price, than we should update
	if closePrice.Compare(currentBuyPrice) < 0 || nextSupportPrice.Compare(s.currentSupportPrice) > 0 {
		logrus.Infof("setup next support take profit price at %f", nextSupportPrice.Float64())
		s.currentSupportPrice = nextSupportPrice
		return true
	}

	return false
}

func findPossibleSupportPrices(closePrice float64, groupDistance float64, lows []float64) []float64 {
	return floats.Group(floats.Lower(lows, closePrice), groupDistance)
}
