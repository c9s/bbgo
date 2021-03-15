package grid

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
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

	TradeService *service.TradeService `json:"-" yaml:"-"`

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol" yaml:"symbol"`

	// ProfitSpread is the fixed profit spread you want to submit the sell order
	ProfitSpread fixedpoint.Value `json:"profitSpread" yaml:"profitSpread"`

	// GridNum is the grid number, how many orders you want to post on the orderbook.
	GridNum int `json:"gridNumber" yaml:"gridNumber"`

	UpperPrice fixedpoint.Value `json:"upperPrice" yaml:"upperPrice"`

	LowerPrice fixedpoint.Value `json:"lowerPrice" yaml:"lowerPrice"`

	// Quantity is the quantity you want to submit for each order.
	Quantity      fixedpoint.Value       `json:"quantity,omitempty"`
	ScaleQuantity *bbgo.PriceVolumeScale `json:"scaleQuantity,omitempty"`

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

	groupID int64
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) generateGridSellOrders(session *bbgo.ExchangeSession) ([]types.SubmitOrder, error) {
	currentPriceFloat, ok := session.LastPrice(s.Symbol)
	if !ok {
		return nil, fmt.Errorf("%s last price not found, skipping", s.Symbol)
	}

	currentPrice := fixedpoint.NewFromFloat(currentPriceFloat)
	priceRange := s.UpperPrice - s.LowerPrice
	if priceRange <= 0 {
		return nil, fmt.Errorf("upper price %f should not be less than or equal to lower price %f", s.UpperPrice.Float64(), s.LowerPrice.Float64())
	}

	numGrids := fixedpoint.NewFromInt(s.GridNum)
	gridSpread := priceRange.Div(numGrids)
	startPrice := fixedpoint.Max(s.LowerPrice, currentPrice+gridSpread)

	if startPrice > s.UpperPrice {
		return nil, fmt.Errorf("current price %f exceeded the upper price boundary %f",
			currentPrice.Float64(),
			s.UpperPrice.Float64())
	}

	balances := session.Account.Balances()
	baseBalance, ok := balances[s.Market.BaseCurrency]
	if !ok {
		return nil, fmt.Errorf("base balance %s not found", s.Market.BaseCurrency)
	}

	if baseBalance.Available == 0 {
		return nil, fmt.Errorf("base balance %s is zero: %+v", s.Market.BaseCurrency, baseBalance)
	}

	log.Infof("placing grid sell orders from %f ~ %f, grid spread %f",
		startPrice.Float64(),
		s.UpperPrice.Float64(),
		gridSpread.Float64())

	var orders []types.SubmitOrder
	for price := startPrice; price <= s.UpperPrice; price += gridSpread {
		var quantity fixedpoint.Value
		if s.Quantity > 0 {
			quantity = s.Quantity
		} else if s.ScaleQuantity != nil {
			qf, err := s.ScaleQuantity.Scale(price.Float64(), 0)
			if err != nil {
				return nil, err
			}
			quantity = fixedpoint.NewFromFloat(qf)
		} else if s.FixedAmount > 0 {
			quantity = s.FixedAmount.Div(price)
		}

		// quoteQuantity := price.Mul(quantity)
		if baseBalance.Available < quantity {
			return orders, fmt.Errorf("base balance %s %f is not enough, stop generating sell orders",
				baseBalance.Currency,
				baseBalance.Available.Float64())
		}

		orders = append(orders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    quantity.Float64(),
			Price:       price.Float64(),
			TimeInForce: "GTC",
			GroupID:     s.groupID,
		})
		baseBalance.Available -= quantity
	}

	return orders, nil
}

func (s *Strategy) generateGridBuyOrders(session *bbgo.ExchangeSession) ([]types.SubmitOrder, error) {
	currentPriceFloat, ok := session.LastPrice(s.Symbol)
	if !ok {
		return nil, fmt.Errorf("%s last price not found, skipping", s.Symbol)
	}

	currentPrice := fixedpoint.NewFromFloat(currentPriceFloat)
	priceRange := s.UpperPrice - s.LowerPrice
	if priceRange <= 0 {
		return nil, fmt.Errorf("upper price %f should not be less than or equal to lower price %f", s.UpperPrice.Float64(), s.LowerPrice.Float64())
	}

	numGrids := fixedpoint.NewFromInt(s.GridNum)
	gridSpread := priceRange.Div(numGrids)
	startPrice := fixedpoint.Min(s.UpperPrice, currentPrice-gridSpread)

	if startPrice < s.LowerPrice {
		return nil, fmt.Errorf("current price %f exceeded the lower price boundary %f",
			currentPrice.Float64(),
			s.UpperPrice.Float64())
	}

	balances := session.Account.Balances()
	balance, ok := balances[s.Market.QuoteCurrency]
	if !ok {
		return nil, fmt.Errorf("quote balance %s not found", s.Market.QuoteCurrency)
	}

	if balance.Available == 0 {
		return nil, fmt.Errorf("quote balance %s is zero: %+v", s.Market.QuoteCurrency, balance)
	}

	log.Infof("placing grid buy orders from %f to %f, grid spread %f",
		(currentPrice - gridSpread).Float64(),
		s.LowerPrice.Float64(),
		gridSpread.Float64())

	var orders []types.SubmitOrder
	for price := startPrice; s.LowerPrice <= price; price -= gridSpread {
		var quantity fixedpoint.Value
		if s.Quantity > 0 {
			quantity = s.Quantity
		} else if s.ScaleQuantity != nil {
			qf, err := s.ScaleQuantity.Scale(price.Float64(), 0)
			if err != nil {
				return nil, err
			}
			quantity = fixedpoint.NewFromFloat(qf)
		} else if s.FixedAmount > 0 {
			quantity = s.FixedAmount.Div(price)
		}

		quoteQuantity := price.Mul(quantity)
		if balance.Available < quoteQuantity {
			return orders, fmt.Errorf("quote balance %s %f is not enough for %f, stop generating buy orders",
				balance.Currency,
				balance.Available.Float64(),
				quoteQuantity.Float64())
		}

		orders = append(orders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    quantity.Float64(),
			Price:       price.Float64(),
			TimeInForce: "GTC",
			GroupID:     s.groupID,
		})
		balance.Available -= quoteQuantity
	}

	return orders, nil
}

func (s *Strategy) placeGridOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	log.Infof("placing grid orders...")

	sellOrders, err := s.generateGridSellOrders(session)
	if err != nil {
		log.Warn(err.Error())
	}
	if len(sellOrders) > 0 {
		createdSellOrders, err := orderExecutor.SubmitOrders(context.Background(), sellOrders...)
		if err != nil {
			log.WithError(err).Error(err.Error())
		} else {
			s.activeOrders.Add(createdSellOrders...)
		}
	}

	buyOrders, err := s.generateGridBuyOrders(session)
	if err != nil {
		log.Warn(err.Error())
	}

	if len(buyOrders) > 0 {
		createdBuyOrders, err := orderExecutor.SubmitOrders(context.Background(), buyOrders...)
		if err != nil {
			log.WithError(err).Error(err.Error())
		} else {
			s.activeOrders.Add(createdBuyOrders...)
		}
	}
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
		GroupID:     s.groupID,
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
	// do some basic validation
	if s.GridNum == 0 {
		s.GridNum = 10
	}

	if s.UpperPrice <= s.LowerPrice {
		return fmt.Errorf("upper price (%f) should not be less than lower price (%f)", s.UpperPrice.Float64(), s.LowerPrice.Float64())
	}

	position, ok := session.Position(s.Symbol)
	if !ok {
		return fmt.Errorf("position not found")
	}

	log.Infof("position: %+v", position)

	instanceID := fmt.Sprintf("grid-%s-%d", s.Symbol, s.GridNum)
	s.groupID = generateGroupID(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

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
	session.Stream.OnStart(func() {
		s.placeGridOrders(orderExecutor, session)
	})

	return nil
}

func generateGroupID(s string) int64 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int64(h.Sum32())
}
