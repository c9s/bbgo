package emastop

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "emastop"

var log = logrus.WithField("strategy", ID)

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	SourceExchangeName string `json:"sourceExchange"`

	TargetExchangeName string `json:"targetExchange"`

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol"`

	// Interval is the interval of the kline channel we want to subscribe,
	// the kline event will trigger the strategy to check if we need to submit order.
	Interval types.Interval `json:"interval"`

	Quantity fixedpoint.Value `json:"quantity"`

	BalancePercentage fixedpoint.Value `json:"balancePercentage"`

	OrderType string `json:"orderType"`

	PriceRatio fixedpoint.Value `json:"priceRatio"`

	StopPriceRatio fixedpoint.Value `json:"stopPriceRatio"`

	// MovingAverageType is the moving average indicator type that we want to use,
	// it could be SMA or EWMA
	MovingAverageType string `json:"movingAverageType"`

	// MovingAverageInterval is the interval of k-lines for the moving average indicator to calculate,
	// it could be "1m", "5m", "1h" and so on.  note that, the moving averages are calculated from
	// the k-line data we subscribed
	MovingAverageInterval types.Interval `json:"movingAverageInterval"`

	// MovingAverageWindow is the number of the window size of the moving average indicator.
	// The number of k-lines in the window. generally used window sizes are 7, 25 and 99 in the TradingView.
	MovingAverageWindow int `json:"movingAverageWindow"`

	order types.Order
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.MovingAverageInterval})
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	sourceSession := sessions[s.SourceExchangeName]
	s.Subscribe(sourceSession)

	// make sure we have the connection alive
	targetSession := sessions[s.TargetExchangeName]
	targetSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) clear(ctx context.Context, orderExecutor bbgo.OrderExecutor) {
	if s.order.OrderID > 0 {
		if err := orderExecutor.CancelOrders(ctx, s.order); err != nil {
			log.WithError(err).Errorf("can not cancel trailingstop order: %+v", s.order)
		}

		// clear out the existing order
		s.order = types.Order{}
	}
}

func (s *Strategy) place(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession, indicator types.Float64Indicator, closePrice fixedpoint.Value) {
	closePriceF := closePrice.Float64()
	movingAveragePriceF := indicator.Last(0)

	// skip it if it's near zero because it's not loaded yet
	if movingAveragePriceF < 0.0001 {
		log.Warnf("moving average price is near 0: %f", movingAveragePriceF)
		return
	}

	// place stop limit order only when the closed price is greater than the moving average price
	if closePriceF <= movingAveragePriceF {
		log.Warnf("close price %v is less than moving average price %f", closePrice, movingAveragePriceF)
		return
	}

	movingAveragePrice := fixedpoint.NewFromFloat(movingAveragePriceF)

	var price = fixedpoint.Zero
	var orderType = types.OrderTypeStopMarket

	switch strings.ToLower(s.OrderType) {
	case "market":
		orderType = types.OrderTypeStopMarket
	case "limit":
		orderType = types.OrderTypeStopLimit
		price = movingAveragePrice
		if s.PriceRatio.Sign() > 0 {
			price = price.Mul(s.PriceRatio)
		}
	}

	market, ok := session.Market(s.Symbol)
	if !ok {
		log.Errorf("market not found, symbol %s", s.Symbol)
		return
	}

	quantity := s.Quantity
	if s.BalancePercentage.Sign() > 0 {

		if balance, ok := session.GetAccount().Balance(market.BaseCurrency); ok {
			quantity = balance.Available.Mul(s.BalancePercentage)
		}
	}

	amount := quantity.Mul(closePrice)
	if amount.Compare(market.MinNotional) < 0 {
		log.Errorf("the amount of stop order (%v) is less than min notional %v", amount, market.MinNotional)
		return
	}

	var stopPrice = movingAveragePrice
	if s.StopPriceRatio.Sign() > 0 {
		stopPrice = stopPrice.Mul(s.StopPriceRatio)
	}

	log.Infof("placing trailingstop order %s at stop price %v, quantity %v", s.Symbol, stopPrice, quantity)

	retOrders, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:    s.Symbol,
		Side:      types.SideTypeSell,
		Type:      orderType,
		Price:     price,
		StopPrice: stopPrice,
		Quantity:  quantity,
	})
	if err != nil {
		log.WithError(err).Error("submit order error")
	}

	if len(retOrders) > 0 {
		s.order = retOrders[0]
	}
}

func (s *Strategy) handleOrderUpdate(order types.Order) {
	if order.OrderID == s.order.OrderID {
		s.order = order
	}
}

func (s *Strategy) loadIndicator(sourceSession *bbgo.ExchangeSession) (types.Float64Indicator, error) {
	var standardIndicatorSet = sourceSession.StandardIndicatorSet(s.Symbol)
	var iw = types.IntervalWindow{Interval: s.MovingAverageInterval, Window: s.MovingAverageWindow}

	switch strings.ToUpper(s.MovingAverageType) {
	case "SMA":
		return standardIndicatorSet.SMA(iw), nil

	case "EWMA", "EMA":
		return standardIndicatorSet.EWMA(iw), nil

	}

	return nil, fmt.Errorf("unsupported moving average type: %s", s.MovingAverageType)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	indicator, err := s.loadIndicator(session)
	if err != nil {
		return err
	}

	session.UserDataStream.OnOrderUpdate(s.handleOrderUpdate)

	// session.UserDataStream.OnKLineClosed
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		closePrice := kline.Close

		// ok, it's our call, we need to cancel the stop limit order first
		s.clear(ctx, orderExecutor)
		s.place(ctx, orderExecutor, session, indicator, closePrice)
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("canceling trailingstop order...")
		s.clear(ctx, orderExecutor)
	})

	if lastPrice, ok := session.LastPrice(s.Symbol); ok {
		s.place(ctx, orderExecutor, session, indicator, lastPrice)
	}

	return nil
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	// source session
	sourceSession := sessions[s.SourceExchangeName]

	// target exchange
	session := sessions[s.TargetExchangeName]
	orderExecutor := bbgo.ExchangeOrderExecutor{
		Session: session,
	}

	indicator, err := s.loadIndicator(sourceSession)
	if err != nil {
		return err
	}

	session.UserDataStream.OnOrderUpdate(s.handleOrderUpdate)

	// session.UserDataStream.OnKLineClosed
	sourceSession.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol || kline.Interval != s.Interval {
			return
		}

		closePrice := kline.Close

		// ok, it's our call, we need to cancel the stop limit order first
		s.clear(ctx, &orderExecutor)
		s.place(ctx, &orderExecutor, session, indicator, closePrice)
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		log.Infof("canceling trailingstop order...")
		s.clear(ctx, &orderExecutor)
	})

	if lastPrice, ok := session.LastPrice(s.Symbol); ok {
		s.place(ctx, &orderExecutor, session, indicator, lastPrice)
	}

	return nil
}
