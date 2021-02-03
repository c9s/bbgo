package grid

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "grid"

var log = logrus.WithField("strategy", ID)

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	// The notification system will be injected into the strategy automatically.
	// This field will be injected automatically since it's a single exchange strategy.
	*bbgo.Notifiability `json:"-" yaml:"-"`

	*bbgo.Graceful `json:"-" yaml:"-"`

	// OrderExecutor is an interface for submitting order.
	// This field will be injected automatically since it's a single exchange strategy.
	bbgo.OrderExecutor `json:"-" yaml:"-"`

	// Market stores the configuration of the market, for example, VolumePrecision, PricePrecision, MinLotSize... etc
	// This field will be injected automatically since we defined the Symbol field.
	types.Market `json:"-" yaml:"-"`

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol" yaml:"symbol"`

	// ProfitSpread is the fixed profit spread you want to submit the sell order
	ProfitSpread fixedpoint.Value `json:"profitSpread" yaml:"profitSpread"`

	// GridNum is the grid number, how many orders you want to post on the orderbook.
	GridNum int `json:"gridNumber" yaml:"gridNumber"`

	UpperPrice fixedpoint.Value `json:"upperPrice" yaml:"upperPrice"`

	LowerPrice fixedpoint.Value `json:"lowerPrice" yaml:"lowerPrice"`

	// Quantity is the quantity you want to submit for each order.
	Quantity float64 `json:"quantity,omitempty"`

	// FixedAmount is used for fixed amount (dynamic quantity) if you don't want to use fixed quantity.
	FixedAmount fixedpoint.Value `json:"amount,omitempty" yaml:"amount"`

	// Long means you want to hold more base asset than the quote asset.
	Long bool `json:"long,omitempty" yaml:"long,omitempty"`

	orderStore *bbgo.OrderStore

	// activeOrders is the locally maintained active order book of the maker orders.
	activeOrders *bbgo.LocalActiveOrderBook

	position fixedpoint.Value

	// any created orders for tracking trades
	orders map[uint64]types.Order
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) placeGridOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	log.Infof("placing grid orders...")

	quoteCurrency := s.Market.QuoteCurrency
	balances := session.Account.Balances()

	currentPrice, ok := session.LastPrice(s.Symbol)
	if !ok {
		log.Warn("last price not found, skipping")
		return
	}

	currentPriceF := fixedpoint.NewFromFloat(currentPrice)
	priceRange := s.UpperPrice - s.LowerPrice
	gridSize := priceRange.Div(fixedpoint.NewFromInt(s.GridNum))

	var bidOrders []types.SubmitOrder
	var askOrders []types.SubmitOrder

	baseBalance, ok := balances[s.Market.BaseCurrency]
	if ok && baseBalance.Available > 0 {
		log.Infof("placing sell order from %f ~ %f per grid %f", (currentPriceF + gridSize).Float64(), s.UpperPrice.Float64(), gridSize.Float64())
		for price := currentPriceF + gridSize; price <= s.UpperPrice; price += gridSize {
			order := types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        types.SideTypeSell,
				Type:        types.OrderTypeLimit,
				Market:      s.Market,
				Quantity:    s.Quantity,
				Price:       price.Float64(),
				TimeInForce: "GTC",
			}
			askOrders = append(askOrders, order)
		}
	} else {
		log.Warnf("base balance is not enough, we can't place ask orders")
	}

	quoteBalance, ok := balances[quoteCurrency]
	if ok && quoteBalance.Available > 0 {
		log.Infof("placing buy order from %f ~ %f per grid %f", (currentPriceF - gridSize).Float64(), s.LowerPrice.Float64(), gridSize.Float64())

		for price := currentPriceF - gridSize; price >= s.LowerPrice; price -= gridSize {
			order := types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        types.SideTypeBuy,
				Type:        types.OrderTypeLimit,
				Market:      s.Market,
				Quantity:    s.Quantity,
				Price:       price.Float64(),
				TimeInForce: "GTC",
			}
			bidOrders = append(bidOrders, order)
		}
	} else {
		log.Warnf("quote balance is not enough, we can't place bid orders")
	}

	createdOrders, err := orderExecutor.SubmitOrders(context.Background(), append(bidOrders, askOrders...)...)
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
	var quantity = order.Quantity

	switch side {
	case types.SideTypeSell:
		price += s.ProfitSpread.Float64()
	case types.SideTypeBuy:
		price -= s.ProfitSpread.Float64()
	}

	if s.FixedAmount > 0 {
		quantity = s.FixedAmount.Float64() / price
	} else if s.Long {
		// long = use the same amount to buy more quantity back
		// the original amount
		var amount = order.Price * order.Quantity
		quantity = amount / price
	}

	submitOrder := types.SubmitOrder{
		Symbol:      s.Symbol,
		Side:        side,
		Type:        types.OrderTypeLimit,
		Quantity:    quantity,
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

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.GridNum == 0 {
		s.GridNum = 10
	}

	if s.UpperPrice <= s.LowerPrice {
		return fmt.Errorf("upper price (%f) should not be less than lower price (%f)", s.UpperPrice.Float64(), s.LowerPrice.Float64())
	}

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.Stream)

	// we don't persist orders so that we can not clear the previous orders for now. just need time to support this.
	s.activeOrders = bbgo.NewLocalActiveOrderBook()
	s.activeOrders.OnFilled(s.submitReverseOrder)
	s.activeOrders.BindStream(session.Stream)

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		log.Infof("canceling active orders...")

		if err := session.Exchange.CancelOrders(ctx, s.activeOrders.Orders()...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}
	})

	session.Stream.OnTradeUpdate(s.tradeUpdateHandler)
	session.Stream.OnConnect(func() {
		s.placeGridOrders(orderExecutor, session)
	})

	return nil
}
