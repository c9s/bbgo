package grid

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("strategy", "grid")

// The indicators (SMA and EWMA) that we want to use are returning float64 data.
type Float64Indicator interface {
	Last() float64
}

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy("grid", &Strategy{})
}

type Strategy struct {
	// The notification system will be injected into the strategy automatically.
	// This field will be injected automatically since it's a single exchange strategy.
	*bbgo.Notifiability

	// OrderExecutor is an interface for submitting order.
	// This field will be injected automatically since it's a single exchange strategy.
	bbgo.OrderExecutor

	// if Symbol string field is defined, bbgo will know it's a symbol-based strategy
	// The following embedded fields will be injected with the corresponding instances.

	// MarketDataStore is a pointer only injection field. public trades, k-lines (candlestick)
	// and order book updates are maintained in the market data store.
	// This field will be injected automatically since we defined the Symbol field.
	*bbgo.MarketDataStore

	// StandardIndicatorSet contains the standard indicators of a market (symbol)
	// This field will be injected automatically since we defined the Symbol field.
	*bbgo.StandardIndicatorSet

	// Market stores the configuration of the market, for example, VolumePrecision, PricePrecision, MinLotSize... etc
	// This field will be injected automatically since we defined the Symbol field.
	types.Market

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol"`

	Interval types.Interval `json:"interval"`

	// GridPips is the pips of grid, e.g., 0.001
	GridPips fixedpoint.Value `json:"gridPips"`

	// GridNum is the grid number (order numbers)
	GridNum int `json:"gridNumber"`

	BaseQuantity float64 `json:"baseQuantity"`

	activeBidOrders *types.SyncOrderMap
	activeAskOrders *types.SyncOrderMap

	boll *indicator.BOLL
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// currently we need the 1m kline to update the last close price and indicators
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval.String()})
}

func (s *Strategy) updateBidOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	quoteCurrency := s.Market.QuoteCurrency
	balances := session.Account.Balances()

	balance, ok := balances[quoteCurrency]
	if !ok || balance.Available <= 0.0 {
		return
	}

	var numOrders = s.GridNum - s.activeBidOrders.Len()
	if numOrders <= 0 {
		return
	}

	var downBand = s.boll.LastDownBand()
	var startPrice = downBand

	var submitOrders []types.SubmitOrder
	for i := 0; i < numOrders; i++ {
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    s.BaseQuantity,
			Price:       startPrice,
			TimeInForce: "GTC",
		})

		startPrice -= s.GridPips.Float64()
	}

	orders, err := orderExecutor.SubmitOrders(context.Background(), submitOrders...)
	if err != nil {
		log.WithError(err).Error("submit bid order error")
		return
	}

	for _, o := range orders {
		log.Infof("adding order %d to the active bid order pool...", o.OrderID)
		s.activeBidOrders.Add(o)
	}
}

func (s *Strategy) updateAskOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	baseCurrency := s.Market.BaseCurrency
	balances := session.Account.Balances()

	balance, ok := balances[baseCurrency]
	if !ok || balance.Available <= 0.0 {
		return
	}

	var numOrders = s.GridNum - s.activeAskOrders.Len()
	if numOrders <= 0 {
		return
	}

	var upBand = s.boll.LastUpBand()
	var startPrice = upBand

	var submitOrders []types.SubmitOrder
	for i := 0; i < numOrders; i++ {
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    s.BaseQuantity,
			Price:       startPrice,
			TimeInForce: "GTC",
		})

		startPrice += s.GridPips.Float64()
	}

	orders, err := orderExecutor.SubmitOrders(context.Background(), submitOrders...)
	if err != nil {
		log.WithError(err).Error("submit ask order error")
		return
	}

	for _, o := range orders {
		log.Infof("adding order %d to the active ask order pool...", o.OrderID)
		s.activeAskOrders.Add(o)
	}
}

func (s *Strategy) updateOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	log.Infof("checking grid orders, bids=%d asks=%d", s.activeBidOrders.Len(), s.activeAskOrders.Len())

	for _, o := range s.activeBidOrders.Orders() {
		log.Infof("bid order: %d -> %s", o.OrderID, o.Status)
	}

	for _, o := range s.activeAskOrders.Orders() {
		log.Infof("ask order: %d -> %s", o.OrderID, o.Status)
	}

	if s.activeBidOrders.Len() < s.GridNum {
		log.Infof("active bid orders not enough: %d < %d, updating...", s.activeBidOrders.Len(), s.GridNum)
		s.updateBidOrders(orderExecutor, session)
	}

	if s.activeAskOrders.Len() < s.GridNum {
		log.Infof("active ask orders not enough: %d < %d, updating...", s.activeAskOrders.Len(), s.GridNum)
		s.updateAskOrders(orderExecutor, session)
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.GridNum == 0 {
		s.GridNum = 2
	}

	s.boll = s.StandardIndicatorSet.GetBOLL(types.IntervalWindow{
		Interval: s.Interval,
		Window:   21,
	})

	// we don't persist orders so that we can not clear the previous orders for now. just need time to support this.
	// TODO: pull this map out and add mutex lock
	s.activeBidOrders = types.NewSyncOrderMap()
	s.activeAskOrders = types.NewSyncOrderMap()

	session.Stream.OnOrderUpdate(func(order types.Order) {
		log.Infof("received order update: %+v", order)

		if order.Symbol != s.Symbol {
			return
		}

		switch order.Status {

		case types.OrderStatusFilled:
			switch order.Side {
			case types.SideTypeSell:
				// find the filled bid to remove
				if filledOrder, ok := s.activeBidOrders.AnyFilled(); ok {
					s.activeBidOrders.Delete(filledOrder.OrderID)
					s.activeAskOrders.Delete(order.OrderID)
				}

			case types.SideTypeBuy:
				// find the filled ask order to remove
				if filledOrder, ok := s.activeAskOrders.AnyFilled(); ok {
					s.activeAskOrders.Delete(filledOrder.OrderID)
					s.activeBidOrders.Delete(order.OrderID)
				}
			}

		case types.OrderStatusCanceled, types.OrderStatusRejected:
			log.Infof("order status %s, removing %d from the active order pool...", order.Status, order.OrderID)

			switch order.Side {
			case types.SideTypeSell:
				s.activeAskOrders.Delete(order.OrderID)
			case types.SideTypeBuy:
				s.activeBidOrders.Delete(order.OrderID)

			}

		default:
			log.Infof("order status %s, updating %d to the active order pool...", order.Status, order.OrderID)
			switch order.Side {
			case types.SideTypeSell:
				s.activeAskOrders.Add(order)
			case types.SideTypeBuy:
				s.activeBidOrders.Add(order)
			}
		}
	})

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		s.updateOrders(orderExecutor, session)

		defer func() {
			for _, o := range s.activeBidOrders.Orders() {
				_ = session.Exchange.CancelOrders(context.Background(), o)
			}

			for _, o := range s.activeAskOrders.Orders() {
				_ = session.Exchange.CancelOrders(context.Background(), o)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				// see if we have enough balances and then we create limit orders on the up band and the down band.
				s.updateOrders(orderExecutor, session)
			}
		}
	}()

	return nil
}
