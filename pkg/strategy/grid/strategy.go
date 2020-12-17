package grid

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("strategy", "grid")

var position int64 = 0

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

	*bbgo.Graceful

	// OrderExecutor is an interface for submitting order.
	// This field will be injected automatically since it's a single exchange strategy.
	bbgo.OrderExecutor


	orderStore *bbgo.OrderStore

	// Market stores the configuration of the market, for example, VolumePrecision, PricePrecision, MinLotSize... etc
	// This field will be injected automatically since we defined the Symbol field.
	types.Market

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol"`

	// ProfitSpread is the fixed profit spread you want to submit the sell order
	ProfitSpread fixedpoint.Value `json:"profitSpread"`

	// GridNum is the grid number, how many orders you want to post on the orderbook.
	GridNum int `json:"gridNumber"`

	UpperPrice fixedpoint.Value `json:"upperPrice"`

	LowerPrice fixedpoint.Value `json:"lowerPrice"`

	// Quantity is the quantity you want to submit for each order.
	Quantity float64 `json:"quantity"`

	// activeOrders is the locally maintained active order book of the maker orders.
	activeOrders *bbgo.LocalActiveOrderBook

	position fixedpoint.Value

	// any created orders for tracking trades
	orders map[uint64]types.Order
}

func (s *Strategy) placeGridOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	quoteCurrency := s.Market.QuoteCurrency
	balances := session.Account.Balances()

	balance, ok := balances[quoteCurrency]
	if !ok || balance.Available <= 0 {
		return
	}

	currentPrice, ok := session.LastPrice(s.Symbol)
	if !ok {
		return
	}

	currentPriceF := fixedpoint.NewFromFloat(currentPrice)
	priceRange := s.UpperPrice - s.LowerPrice
	gridSize := priceRange.Div(fixedpoint.NewFromInt(s.GridNum))

	log.Infof("current price: %f", currentPrice)

	var orders []types.SubmitOrder
	for price := s.LowerPrice; price <= s.UpperPrice; price += gridSize {
		var side types.SideType
		if price > currentPriceF {
			side = types.SideTypeSell
		} else {
			side = types.SideTypeBuy
		}

		order := types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        side,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    s.Quantity,
			Price:       price.Float64(),
			TimeInForce: "GTC",
		}
		log.Infof("submitting order: %s", order.String())
		orders = append(orders, order)
	}

	createdOrders, err := orderExecutor.SubmitOrders(context.Background(), orders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
		return
	}

	s.activeOrders.Add(createdOrders...)
}

func (s *Strategy) tradeUpdateHandler(trade types.Trade) {
	if trade.Symbol != s.Symbol {
		return
	}

	if s.orderStore.Exists(trade.OrderID) {
		log.Infof("received trade update of order %d: %+v", trade.OrderID, trade)
		switch trade.Side {
		case types.SideTypeBuy:
			s.position.AtomicAdd(fixedpoint.NewFromFloat(trade.Quantity))
		case types.SideTypeSell:
			s.position.AtomicAdd(-fixedpoint.NewFromFloat(trade.Quantity))
		}
	}
}

func (s *Strategy) submitReverseOrder(order types.Order) {
	var side = order.Side.Reverse()
	var price = order.Price

	switch side {
	case types.SideTypeSell:
		price += s.ProfitSpread.Float64()

	case types.SideTypeBuy:
		price -= s.ProfitSpread.Float64()

	}

	submitOrder := types.SubmitOrder{
		Symbol:      s.Symbol,
		Side:        side,
		Type:        types.OrderTypeLimit,
		Quantity:    order.Quantity,
		Price:       price,
		TimeInForce: "GTC",
	}

	log.Infof("submitting reverse order: %s against %s", submitOrder.String(), order.String())

	createdOrders, err := s.OrderExecutor.SubmitOrders(context.Background(), submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
		return
	}

	s.orderStore.Add(createdOrders...)
	s.activeOrders.Add(createdOrders...)
}

func (s *Strategy) orderUpdateHandler(order types.Order) {
	if order.Symbol != s.Symbol {
		return
	}

	log.Infof("order update: %s", order.String())

	switch order.Status {
	case types.OrderStatusFilled:
		s.activeOrders.Remove(order)
		s.submitReverseOrder(order)

	case types.OrderStatusPartiallyFilled, types.OrderStatusNew:
		s.activeOrders.Update(order)

	case types.OrderStatusCanceled, types.OrderStatusRejected:
		log.Infof("order status %s, removing %d from the active order pool...", order.Status, order.OrderID)
		s.activeOrders.Remove(order)
	}
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.GridNum == 0 {
		s.GridNum = 10
	}

	s.orderStore = bbgo.NewOrderStore()
	s.orderStore.BindStream(session.Stream)

	// we don't persist orders so that we can not clear the previous orders for now. just need time to support this.
	s.activeOrders = bbgo.NewLocalActiveOrderBook()

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		log.Infof("canceling active orders...")

		if err := session.Exchange.CancelOrders(ctx, s.activeOrders.Orders()...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}
	})

	session.Stream.OnOrderUpdate(s.orderUpdateHandler)
	session.Stream.OnTradeUpdate(s.tradeUpdateHandler)
	session.Stream.OnConnect(func() {
		s.placeGridOrders(orderExecutor, session)
	})

	return nil
}
