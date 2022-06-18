package infinity_grid

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/sirupsen/logrus"
)

const ID = "infinity-grid"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// State is the grid snapshot
type State struct {
	Orders          []types.SubmitOrder           `json:"orders,omitempty"`
	FilledBuyGrids  map[fixedpoint.Value]struct{} `json:"filledBuyGrids"`
	FilledSellGrids map[fixedpoint.Value]struct{} `json:"filledSellGrids"`
	Position        *types.Position               `json:"position,omitempty"`

	ProfitStats types.ProfitStats `json:"profitStats,omitempty"`
}

type Strategy struct {
	// The notification system will be injected into the strategy automatically.
	// This field will be injected automatically since it's a single exchange strategy.
	*bbgo.Notifiability `json:"-" yaml:"-"`

	*bbgo.Graceful `json:"-" yaml:"-"`

	*bbgo.Persistence

	// OrderExecutor is an interface for submitting order.
	// This field will be injected automatically since it's a single exchange strategy.
	bbgo.OrderExecutor `json:"-" yaml:"-"`

	// Market stores the configuration of the market, for example, VolumePrecision, PricePrecision, MinLotSize... etc
	// This field will be injected automatically since we defined the Symbol field.
	types.Market `json:"-" yaml:"-"`

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol" yaml:"symbol"`

	LowerPrice fixedpoint.Value `json:"lowerPrice" yaml:"lowerPrice"`

	// Buy-Sell Margin for each pair of orders
	Margin fixedpoint.Value `json:"margin"`

	// Quantity is the quantity you want to submit for each order.
	Quantity fixedpoint.Value `json:"quantity"`

	InitialOrderQuantity fixedpoint.Value `json:"initialOrderQuantity"`
	CountOfMoreOrders    int              `json:"countOfMoreOrders"`

	// GridNum is the grid number, how many orders you want to post on the orderbook.
	GridNum int64 `json:"gridNumber" yaml:"gridNumber"`

	// Side is the initial maker orders side. defaults to "both"
	Side types.SideType `json:"side" yaml:"side"`

	// Long means you want to hold more base asset than the quote asset.
	Long bool `json:"long,omitempty" yaml:"long,omitempty"`

	state *State

	// orderStore is used to store all the created orders, so that we can filter the trades.
	orderStore *bbgo.OrderStore

	// activeOrders is the locally maintained active order book of the maker orders.
	activeOrders *bbgo.ActiveOrderBook

	tradeCollector *bbgo.TradeCollector

	currentUpperGrid int
	currentLowerGrid int

	// groupID is the group ID used for the strategy instance for canceling orders
	groupID uint32
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.LowerPrice.IsZero() {
		return errors.New("lowerPrice can not be zero, you forgot to set?")
	}

	if s.Margin.Sign() <= 0 {
		// If margin is empty or its value is negative
		return fmt.Errorf("Margin should bigger than 0")
	}

	if s.Quantity.IsZero() {
		return fmt.Errorf("Quantity can not be zero")
	}

	return nil
}

func (s *Strategy) placeInfiniteGridOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	balances := session.Account.Balances()
	log.Infof("Balances: %s", balances.String())
	log.Infof("Base currency: %s", s.Market.BaseCurrency)   // BTC
	log.Infof("Quote currency: %s", s.Market.QuoteCurrency) // USD
	baseBalance, ok := balances[s.Market.BaseCurrency]
	if !ok {
		log.Errorf("base balance %s not found", s.Market.BaseCurrency)
		return
	}
	if s.currentUpperGrid != 0 || s.currentLowerGrid != 0 {
		// reconnect, do not place orders
		return
	}

	quoteBalance, ok := balances[s.Market.QuoteCurrency]
	if !ok || quoteBalance.Available.Compare(fixedpoint.Zero) < 0 { // check available USD in balance
		log.Errorf("quote balance %s not found", s.Market.QuoteCurrency)
		return
	}

	var orders []types.SubmitOrder
	var quantityF fixedpoint.Value
	currentPrice, ok := session.LastPrice(s.Symbol)
	if !ok {
		return
	}

	quantityF = s.Quantity
	if s.InitialOrderQuantity.Compare(fixedpoint.Zero) > 0 {
		quantityF = s.InitialOrderQuantity
		// Buy half of value of asset
		order := types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeMarket,
			Market:      s.Market,
			Quantity:    quantityF,
			Price:       currentPrice,
			TimeInForce: types.TimeInForceGTC,
			GroupID:     s.groupID,
		}
		log.Infof("submitting init order: %s", order.String())
		orders = append(orders, order)

		baseBalance.Available = baseBalance.Available.Add(quantityF)
		//createdOrders, err := orderExecutor.SubmitOrders(context.Background(), order)
		//if err != nil {
		//log.WithError(err).Errorf("can not place init order")
		//return
		//}

		//s.activeOrders.Add(createdOrders...)
		//s.orderStore.Add(createdOrders...)
	}

	// Sell Side
	j := 1
	for i := int64(1); i <= s.GridNum/2; i++ {
		price := fixedpoint.NewFromFloat(currentPrice.Float64() * math.Pow((1.0+s.Margin.Float64()), float64(j)))
		j++
		if price.Compare(s.LowerPrice) < 0 {
			i--
			continue
		}

		quantity := s.Quantity
		//quoteQuantity := price.Mul(quantity)
		if baseBalance.Available.Compare(quantity) < 0 {
			log.Errorf("base balance %s %s is not enough, stop generating sell orders",
				baseBalance.Currency,
				baseBalance.Available.String())
			break
		}
		if _, filled := s.state.FilledSellGrids[price]; filled {
			log.Debugf("sell grid at price %v is already filled, skipping", price)
			continue
		}
		order := types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    quantity,
			Price:       price,
			TimeInForce: types.TimeInForceGTC,
			GroupID:     s.groupID,
		}
		log.Infof("%d) submitting order: %s", i, order.String())
		orders = append(orders, order)
		baseBalance.Available = baseBalance.Available.Sub(quantity)

		s.state.FilledSellGrids[price] = struct{}{}
		s.currentUpperGrid++
	}

	// Buy Side
	for i := int64(1); i <= s.GridNum/2; i++ {
		price := fixedpoint.NewFromFloat(currentPrice.Float64() * math.Pow((1.0-s.Margin.Float64()), float64(i)))

		if price.Compare(s.LowerPrice) < 0 {
			break
		}

		quantity := s.Quantity
		quoteQuantity := price.Mul(quantity)
		if quoteBalance.Available.Compare(quoteQuantity) < 0 {
			log.Errorf("quote balance %s %v is not enough for %v, stop generating buy orders",
				quoteBalance.Currency,
				quoteBalance.Available,
				quoteQuantity)
			break
		}
		if _, filled := s.state.FilledBuyGrids[price]; filled {
			log.Debugf("buy grid at price %v is already filled, skipping", price)
			continue
		}
		order := types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    quantity,
			Price:       price,
			TimeInForce: types.TimeInForceGTC,
			GroupID:     s.groupID,
		}
		log.Infof("%d) submitting order: %s", i, order.String())
		orders = append(orders, order)

		quoteBalance.Available = quoteBalance.Available.Sub(quoteQuantity)

		s.state.FilledBuyGrids[price] = struct{}{}
		s.currentLowerGrid++
	}

	createdOrders, err := orderExecutor.SubmitOrders(context.Background(), orders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
		return
	}

	s.activeOrders.Add(createdOrders...)
	s.orderStore.Add(createdOrders...)
}

func (s *Strategy) submitFollowingOrder(order types.Order) {
	var side = order.Side.Reverse()
	var orders []types.SubmitOrder
	var cancelOrders []types.Order
	var price fixedpoint.Value
	var quantity = order.Quantity
	const earlyPlacedCount = 2

	if order.Quantity.Eq(s.InitialOrderQuantity) {
		return
	}

	switch side {
	case types.SideTypeSell:
		price = order.Price.Mul(fixedpoint.NewFromFloat(1.0).Add(s.Margin))
		s.currentUpperGrid++
		s.currentLowerGrid--
		if s.Long {
			quantity = s.Quantity
		}

	case types.SideTypeBuy:
		price = order.Price.Mul(fixedpoint.NewFromFloat(1.0).Sub(s.Margin))
		if price.Compare(s.LowerPrice) < 0 {
			return
		}
		if s.Long {
			var amount = order.Price.Mul(order.Quantity)
			quantity = amount.Div(price)
		}
		s.currentUpperGrid--
		s.currentLowerGrid++
	}

	submitOrder := types.SubmitOrder{
		Symbol:      s.Symbol,
		Side:        side,
		Type:        types.OrderTypeLimit,
		Market:      s.Market,
		Quantity:    quantity,
		Price:       price,
		TimeInForce: types.TimeInForceGTC,
		GroupID:     s.groupID,
	}

	if price.Compare(s.LowerPrice) >= 0 {
		log.Infof("→submitting following order: %s, currentUpperGrid: %d, currentLowerGrid: %d", submitOrder.String(), s.currentUpperGrid, s.currentLowerGrid)
		orders = append(orders, submitOrder)
	}

	if order.Side == types.SideTypeSell && s.currentUpperGrid <= earlyPlacedCount {
		// Plase a more higher order
		for i := 1; i <= s.CountOfMoreOrders; i++ {
			price = order.Price.MulPow(fixedpoint.NewFromFloat(1.0).Add(s.Margin), fixedpoint.NewFromInt(int64(i+earlyPlacedCount)))
			submitOrder := types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        order.Side,
				Market:      s.Market,
				Type:        types.OrderTypeLimit,
				Quantity:    s.Quantity,
				Price:       price,
				TimeInForce: types.TimeInForceGTC,
				GroupID:     s.groupID,
			}

			orders = append(orders, submitOrder)
			s.currentUpperGrid++
			log.Infof("submitting new higher order: %s, currentUpperGrid: %d", submitOrder.String(), s.currentUpperGrid)
		}
		// Cleanup overabundant order limits
		lowerGridPrice := order.Price.MulPow(fixedpoint.NewFromFloat(1.0).Sub(s.Margin), fixedpoint.NewFromInt(int64(s.GridNum)))
		for _, cancelOrder := range s.activeOrders.Orders() {
			if cancelOrder.Side == types.SideTypeSell {
				continue
			}
			if cancelOrder.Price.Compare(lowerGridPrice) < 0 {
				cancelOrders = append(cancelOrders, cancelOrder)
			}
		}
		log.Infof("cleanup %d the lowest orders", len(cancelOrders))
		s.currentLowerGrid -= len(cancelOrders)
		s.OrderExecutor.CancelOrders(context.Background(), cancelOrders...)
	}

	if order.Side == types.SideTypeBuy && s.currentLowerGrid <= earlyPlacedCount {
		// Plase a more lower order
		for i := 1; i <= s.CountOfMoreOrders; i++ {
			price = order.Price.MulPow(fixedpoint.NewFromFloat(1.0).Sub(s.Margin), fixedpoint.NewFromInt(int64(i+earlyPlacedCount)))

			if price.Compare(s.LowerPrice) < 0 {
				break
			}

			submitOrder := types.SubmitOrder{
				Symbol:      s.Symbol,
				Side:        order.Side,
				Market:      s.Market,
				Type:        types.OrderTypeLimit,
				Quantity:    s.Quantity,
				Price:       price,
				TimeInForce: types.TimeInForceGTC,
				GroupID:     s.groupID,
			}

			orders = append(orders, submitOrder)
			s.currentLowerGrid++
			log.Infof("submitting new lower order: %s, currentLowerGrid: %d", submitOrder.String(), s.currentLowerGrid)
		}
		// Cleanup overabundant order limits
		upperGridPrice := order.Price.MulPow(fixedpoint.NewFromFloat(1.0).Add(s.Margin), fixedpoint.NewFromInt(int64(s.GridNum)))
		for _, cancelOrder := range s.activeOrders.Orders() {
			if cancelOrder.Side == types.SideTypeBuy {
				continue
			}
			if cancelOrder.Price.Compare(upperGridPrice) > 0 {
				cancelOrders = append(cancelOrders, cancelOrder)
			}
		}
		log.Infof("cleanup %d the highest orders", len(cancelOrders))
		s.currentUpperGrid -= len(cancelOrders)
		s.OrderExecutor.CancelOrders(context.Background(), cancelOrders...)
	}

	createdOrders, err := s.OrderExecutor.SubmitOrders(context.Background(), orders...)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
		return
	}

	s.activeOrders.Add(createdOrders...)
}

func (s *Strategy) handleFilledOrder(order types.Order) {
	if order.Symbol != s.Symbol {
		return
	}

	//s.Notifiability.Notify("order filled: %s", order.String())
	s.submitFollowingOrder(order)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) LoadState() error {
	instanceID := s.InstanceID()

	var state State
	if s.Persistence != nil {
		if err := s.Persistence.Load(&state, ID, instanceID); err != nil {
			if err != service.ErrPersistenceNotExists {
				return errors.Wrapf(err, "state load error")
			}

			s.state = &State{
				FilledBuyGrids:  make(map[fixedpoint.Value]struct{}),
				FilledSellGrids: make(map[fixedpoint.Value]struct{}),
				Position:        types.NewPositionFromMarket(s.Market),
			}
		} else {
			s.state = &state
		}
	}

	// init profit stats
	s.state.ProfitStats.Init(s.Market)

	// field guards
	if s.state.FilledBuyGrids == nil {
		s.state.FilledBuyGrids = make(map[fixedpoint.Value]struct{})
	}
	if s.state.FilledSellGrids == nil {
		s.state.FilledSellGrids = make(map[fixedpoint.Value]struct{})
	}

	return nil
}

func (s *Strategy) SaveState() error {
	if s.Persistence != nil {
		log.Infof("backing up grid state...")

		instanceID := s.InstanceID()
		s.state.Orders = s.activeOrders.Backup()

		if err := s.Persistence.Save(s.state, ID, instanceID); err != nil {
			return err
		}
	}
	return nil
}

// InstanceID returns the instance identifier from the current grid configuration parameters
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s-%d-%d", ID, s.Symbol, s.GridNum, s.LowerPrice.Int())
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.GridNum == 0 {
		s.GridNum = 10
	}

	instanceID := s.InstanceID()
	s.groupID = util.FNV32(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	if err := s.LoadState(); err != nil {
		return err
	}

	s.Notify("grid %s position", s.Symbol, s.state.Position)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	s.activeOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeOrders.OnFilled(s.handleFilledOrder)
	s.activeOrders.BindStream(session.UserDataStream)

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.state.Position, s.orderStore)

	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		s.Notifiability.Notify(trade)
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		s.Notifiability.Notify(position)
	})
	s.tradeCollector.BindStream(session.UserDataStream)

	s.currentLowerGrid = 0
	s.currentUpperGrid = 0

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if err := s.SaveState(); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		} else {
			s.Notify("%s: %s grid is saved", ID, s.Symbol)
		}

		// now we can cancel the open orders
		log.Infof("canceling %d active orders...", s.activeOrders.NumOfOrders())
		if err := session.Exchange.CancelOrders(ctx, s.activeOrders.Orders()...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}

		//log.Infoln(s.state.ProfitStats.PlainText())
	})
	session.MarketDataStream.OnConnect(func() {})
	session.UserDataStream.OnStart(func() {
		if len(s.state.Orders) > 0 {
			s.Notifiability.Notify("restoring %s %d grid orders...", s.Symbol, len(s.state.Orders))

			createdOrders, err := orderExecutor.SubmitOrders(ctx, s.state.Orders...)
			if err != nil {
				log.WithError(err).Error("active orders restore error")
			}
			s.activeOrders.Add(createdOrders...)
			s.orderStore.Add(createdOrders...)
		} else {
			s.placeInfiniteGridOrders(orderExecutor, session)
		}
	})

	return nil
}
