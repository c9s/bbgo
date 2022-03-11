package grid

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "grid"

var log = logrus.WithField("strategy", ID)

var NotionalModifier = fixedpoint.NewFromFloat(1.0001)

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// State is the grid snapshot
type State struct {
	Orders          []types.SubmitOrder           `json:"orders,omitempty"`
	FilledBuyGrids  map[fixedpoint.Value]struct{} `json:"filledBuyGrids"`
	FilledSellGrids map[fixedpoint.Value]struct{} `json:"filledSellGrids"`
	Position        *types.Position               `json:"position,omitempty"`

	AccumulativeArbitrageProfit fixedpoint.Value `json:"accumulativeArbitrageProfit"`

	// any created orders for tracking trades
	// [source Order ID] -> arbitrage order
	ArbitrageOrders map[uint64]types.Order `json:"arbitrageOrders"`

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

	TradeService *service.TradeService `json:"-" yaml:"-"`

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol" yaml:"symbol"`

	// ProfitSpread is the fixed profit spread you want to submit the sell order
	ProfitSpread fixedpoint.Value `json:"profitSpread" yaml:"profitSpread"`

	// GridNum is the grid number, how many orders you want to post on the orderbook.
	GridNum int64 `json:"gridNumber" yaml:"gridNumber"`

	UpperPrice fixedpoint.Value `json:"upperPrice" yaml:"upperPrice"`

	LowerPrice fixedpoint.Value `json:"lowerPrice" yaml:"lowerPrice"`

	// Quantity is the quantity you want to submit for each order.
	Quantity fixedpoint.Value `json:"quantity,omitempty"`

	// QuantityScale helps user to define the quantity by price scale or volume scale
	QuantityScale *bbgo.PriceVolumeScale `json:"quantityScale,omitempty"`

	// FixedAmount is used for fixed amount (dynamic quantity) if you don't want to use fixed quantity.
	FixedAmount fixedpoint.Value `json:"amount,omitempty" yaml:"amount"`

	// Side is the initial maker orders side. defaults to "both"
	Side types.SideType `json:"side" yaml:"side"`

	// CatchUp let the maker grid catch up with the price change.
	CatchUp bool `json:"catchUp" yaml:"catchUp"`

	// Long means you want to hold more base asset than the quote asset.
	Long bool `json:"long,omitempty" yaml:"long,omitempty"`

	state *State

	// orderStore is used to store all the created orders, so that we can filter the trades.
	orderStore *bbgo.OrderStore

	// activeOrders is the locally maintained active order book of the maker orders.
	activeOrders *bbgo.LocalActiveOrderBook

	tradeCollector *bbgo.TradeCollector

	// groupID is the group ID used for the strategy instance for canceling orders
	groupID uint32
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.UpperPrice.IsZero() {
		return errors.New("upperPrice can not be zero, you forgot to set?")
	}
	if s.LowerPrice.IsZero() {
		return errors.New("lowerPrice can not be zero, you forgot to set?")
	}
	if s.UpperPrice.Compare(s.LowerPrice) <= 0 {
		return fmt.Errorf("upperPrice (%s) should not be less than or equal to lowerPrice (%s)", s.UpperPrice.String(), s.LowerPrice.String())
	}

	if s.ProfitSpread.Sign() <= 0 {
		// If profitSpread is empty or its value is negative
		return fmt.Errorf("profit spread should bigger than 0")
	}

	if s.Quantity.IsZero() && s.QuantityScale == nil && s.FixedAmount.IsZero() {
		return fmt.Errorf("amount, quantity or scaleQuantity can not be zero")
	}

	return nil
}

func (s *Strategy) generateGridSellOrders(session *bbgo.ExchangeSession) ([]types.SubmitOrder, error) {
	currentPrice, ok := session.LastPrice(s.Symbol)
	if !ok {
		return nil, fmt.Errorf("can not generate sell orders, %s last price not found", s.Symbol)
	}

	if currentPrice.Compare(s.UpperPrice) > 0 {
		return nil, fmt.Errorf("can not generate sell orders, the current price %s is higher than upper price %s", currentPrice.String(), s.UpperPrice.String())
	}

	priceRange := s.UpperPrice.Sub(s.LowerPrice)
	numGrids := fixedpoint.NewFromInt(s.GridNum)
	gridSpread := priceRange.Div(numGrids)

	if gridSpread.IsZero() {
		return nil, fmt.Errorf(
			"either numGrids(%v) is too big or priceRange(%v) is too small, "+
				"the differences of grid prices become zero", numGrids, priceRange)
	}

	// find the nearest grid price from the current price
	startPrice := fixedpoint.Max(
		s.LowerPrice,
		s.UpperPrice.Sub(
			s.UpperPrice.Sub(currentPrice).Div(gridSpread).Trunc().Mul(gridSpread)))

	if startPrice.Compare(s.UpperPrice) > 0 {
		return nil, fmt.Errorf("current price %v exceeded the upper price boundary %v",
			currentPrice,
			s.UpperPrice)
	}

	balances := session.Account.Balances()
	baseBalance, ok := balances[s.Market.BaseCurrency]
	if !ok {
		return nil, fmt.Errorf("base balance %s not found", s.Market.BaseCurrency)
	}

	if baseBalance.Available.IsZero() {
		return nil, fmt.Errorf("base balance %s is zero: %s",
			s.Market.BaseCurrency, baseBalance.String())
	}

	log.Infof("placing grid sell orders from %s ~ %s, grid spread %s",
		startPrice.String(),
		s.UpperPrice.String(),
		gridSpread.String())

	var orders []types.SubmitOrder
	for price := startPrice; price.Compare(s.UpperPrice) <= 0; price = price.Add(gridSpread) {
		var quantity fixedpoint.Value
		if s.Quantity.Sign() > 0 {
			quantity = s.Quantity
		} else if s.QuantityScale != nil {
			qf, err := s.QuantityScale.Scale(price.Float64(), 0)
			if err != nil {
				return nil, err
			}
			quantity = fixedpoint.NewFromFloat(qf)
		} else if s.FixedAmount.Sign() > 0 {
			quantity = s.FixedAmount.Div(price)
		}

		// quoteQuantity := price.Mul(quantity)
		if baseBalance.Available.Compare(quantity) < 0 {
			return orders, fmt.Errorf("base balance %s %s is not enough, stop generating sell orders",
				baseBalance.Currency,
				baseBalance.Available.String())
		}

		if _, filled := s.state.FilledSellGrids[price]; filled {
			log.Debugf("sell grid at price %s is already filled, skipping", price.String())
			continue
		}

		orders = append(orders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    quantity,
			Price:       price.Add(s.ProfitSpread),
			TimeInForce: types.TimeInForceGTC,
			GroupID:     s.groupID,
		})
		baseBalance.Available = baseBalance.Available.Sub(quantity)

		s.state.FilledSellGrids[price] = struct{}{}
	}

	return orders, nil
}

func (s *Strategy) generateGridBuyOrders(session *bbgo.ExchangeSession) ([]types.SubmitOrder, error) {
	// session.Exchange.QueryTicker()
	currentPrice, ok := session.LastPrice(s.Symbol)
	if !ok {
		return nil, fmt.Errorf("%s last price not found, skipping", s.Symbol)
	}

	if currentPrice.Compare(s.LowerPrice) < 0 {
		return nil, fmt.Errorf("current price %v is lower than the lower price %v",
			currentPrice, s.LowerPrice)
	}

	priceRange := s.UpperPrice.Sub(s.LowerPrice)
	numGrids := fixedpoint.NewFromInt(s.GridNum)
	gridSpread := priceRange.Div(numGrids)

	if gridSpread.IsZero() {
		return nil, fmt.Errorf(
			"either numGrids(%v) is too big or priceRange(%v) is too small, "+
				"the differences of grid prices become zero", numGrids, priceRange)
	}

	// Find the nearest grid price for placing buy orders:
	// buyRange = currentPrice - lowerPrice
	// numOfBuyGrids = Floor(buyRange / gridSpread)
	// startPrice = lowerPrice + numOfBuyGrids * gridSpread
	// priceOfBuyOrder1 = startPrice
	// priceOfBuyOrder2 = startPrice - gridSpread
	// priceOfBuyOrder3 = startPrice - gridSpread * 2
	startPrice := fixedpoint.Min(
		s.UpperPrice,
		s.LowerPrice.Add(
			currentPrice.Sub(s.LowerPrice).Div(gridSpread).Trunc().Mul(gridSpread)))

	if startPrice.Compare(s.LowerPrice) < 0 {
		return nil, fmt.Errorf("current price %v exceeded the lower price boundary %v",
			currentPrice,
			s.UpperPrice)
	}

	balances := session.Account.Balances()
	balance, ok := balances[s.Market.QuoteCurrency]
	if !ok {
		return nil, fmt.Errorf("quote balance %s not found", s.Market.QuoteCurrency)
	}

	if balance.Available.IsZero() {
		return nil, fmt.Errorf("quote balance %s is zero: %v", s.Market.QuoteCurrency, balance)
	}

	log.Infof("placing grid buy orders from %v to %v, grid spread %v",
		startPrice,
		s.LowerPrice,
		gridSpread)

	var orders []types.SubmitOrder
	for price := startPrice; s.LowerPrice.Compare(price) <= 0; price = price.Sub(gridSpread) {
		var quantity fixedpoint.Value
		if s.Quantity.Sign() > 0 {
			quantity = s.Quantity
		} else if s.QuantityScale != nil {
			qf, err := s.QuantityScale.Scale(price.Float64(), 0)
			if err != nil {
				return nil, err
			}
			quantity = fixedpoint.NewFromFloat(qf)
		} else if s.FixedAmount.Sign() > 0 {
			quantity = s.FixedAmount.Div(price)
		}

		quoteQuantity := price.Mul(quantity)
		if balance.Available.Compare(quoteQuantity) < 0 {
			return orders, fmt.Errorf("quote balance %s %v is not enough for %v, stop generating buy orders",
				balance.Currency,
				balance.Available,
				quoteQuantity)
		}

		if _, filled := s.state.FilledBuyGrids[price]; filled {
			log.Debugf("buy grid at price %v is already filled, skipping", price)
			continue
		}

		orders = append(orders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeLimit,
			Market:      s.Market,
			Quantity:    quantity,
			Price:       price,
			TimeInForce: types.TimeInForceGTC,
			GroupID:     s.groupID,
		})
		balance.Available = balance.Available.Sub(quoteQuantity)

		s.state.FilledBuyGrids[price] = struct{}{}
	}

	return orders, nil
}

func (s *Strategy) placeGridSellOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	orderForms, err := s.generateGridSellOrders(session)

	if len(orderForms) == 0 {
		if err != nil {
			return err
		}

		return errors.New("none of sell order is generated")
	}

	log.Infof("submitting %d sell orders...", len(orderForms))
	createdOrders, err := orderExecutor.SubmitOrders(context.Background(), orderForms...)
	s.activeOrders.Add(createdOrders...)
	return err
}

func (s *Strategy) placeGridBuyOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	orderForms, err := s.generateGridBuyOrders(session)

	if len(orderForms) == 0 {
		if err != nil {
			return err
		}

		return errors.New("none of buy order is generated")
	}

	log.Infof("submitting %d buy orders...", len(orderForms))
	createdOrders, err := orderExecutor.SubmitOrders(context.Background(), orderForms...)
	s.activeOrders.Add(createdOrders...)

	return err
}

func (s *Strategy) placeGridOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	log.Infof("placing grid orders on side %s...", s.Side)

	switch s.Side {

	case types.SideTypeBuy:
		if err := s.placeGridBuyOrders(orderExecutor, session); err != nil {
			log.Warn(err.Error())
		}

	case types.SideTypeSell:
		if err := s.placeGridSellOrders(orderExecutor, session); err != nil {
			log.Warn(err.Error())
		}

	case types.SideTypeBoth:
		if err := s.placeGridSellOrders(orderExecutor, session); err != nil {
			log.Warn(err.Error())
		}

		if err := s.placeGridBuyOrders(orderExecutor, session); err != nil {
			log.Warn(err.Error())
		}

	default:
		log.Errorf("invalid side %s", s.Side)
	}
}

func (s *Strategy) handleFilledOrder(filledOrder types.Order) {
	// generate arbitrage order
	var side = filledOrder.Side.Reverse()
	var price = filledOrder.Price
	var quantity = filledOrder.Quantity
	var amount = filledOrder.Price.Mul(filledOrder.Quantity)

	switch side {
	case types.SideTypeSell:
		price = price.Add(s.ProfitSpread)
	case types.SideTypeBuy:
		price = price.Sub(s.ProfitSpread)
	}

	if s.FixedAmount.Sign() > 0 {
		quantity = s.FixedAmount.Div(price)
	} else if s.Long {
		// long = use the same amount to buy more quantity back
		quantity = amount.Div(price)
		amount = quantity.Mul(price)
	}

	if quantity.Compare(s.Market.MinQuantity) < 0 {
		quantity = s.Market.MinQuantity
		amount = quantity.Mul(price)
	}

	if amount.Compare(s.Market.MinNotional) <= 0 {
		quantity = bbgo.AdjustFloatQuantityByMinAmount(
			quantity, price, s.Market.MinNotional.Mul(NotionalModifier))

		// update amount
		amount = quantity.Mul(price)
	}

	submitOrder := types.SubmitOrder{
		Symbol:      s.Symbol,
		Side:        side,
		Type:        types.OrderTypeLimit,
		Quantity:    quantity,
		Price:       price,
		TimeInForce: types.TimeInForceGTC,
		GroupID:     s.groupID,
	}

	log.Infof("submitting arbitrage order: %v against filled order %v", submitOrder, filledOrder)

	createdOrders, err := s.OrderExecutor.SubmitOrders(context.Background(), submitOrder)

	// create one-way link from the newly created orders
	for _, o := range createdOrders {
		s.state.ArbitrageOrders[o.OrderID] = filledOrder
	}

	s.orderStore.Add(createdOrders...)
	s.activeOrders.Add(createdOrders...)

	if err != nil {
		log.WithError(err).Errorf("can not place orders: %+v", submitOrder)
		return
	}

	// calculate arbitrage profit
	// TODO: apply fee rate here
	if s.Long {
		switch filledOrder.Side {
		case types.SideTypeSell:
			if buyOrder, ok := s.state.ArbitrageOrders[filledOrder.OrderID]; ok {
				// use base asset quantity here
				baseProfit := buyOrder.Quantity.Sub(filledOrder.Quantity)
				s.state.AccumulativeArbitrageProfit = s.state.AccumulativeArbitrageProfit.
					Add(baseProfit)
				s.Notify("%s grid arbitrage profit %v %s, accumulative arbitrage profit %v %s",
					s.Symbol,
					baseProfit, s.Market.BaseCurrency,
					s.state.AccumulativeArbitrageProfit, s.Market.BaseCurrency,
				)
			}

		case types.SideTypeBuy:
			if sellOrder, ok := s.state.ArbitrageOrders[filledOrder.OrderID]; ok {
				// use base asset quantity here
				baseProfit := filledOrder.Quantity.Sub(sellOrder.Quantity)
				s.state.AccumulativeArbitrageProfit = s.state.AccumulativeArbitrageProfit.Add(baseProfit)
				s.Notify("%s grid arbitrage profit %v %s, accumulative arbitrage profit %v %s",
					s.Symbol,
					baseProfit, s.Market.BaseCurrency,
					s.state.AccumulativeArbitrageProfit, s.Market.BaseCurrency,
				)
			}
		}
	} else if !s.Long && s.Quantity.Sign() > 0 {
		switch filledOrder.Side {
		case types.SideTypeSell:
			if buyOrder, ok := s.state.ArbitrageOrders[filledOrder.OrderID]; ok {
				// use base asset quantity here
				quoteProfit := filledOrder.Quantity.Mul(filledOrder.Price).Sub(
					buyOrder.Quantity.Mul(buyOrder.Price))
				s.state.AccumulativeArbitrageProfit = s.state.AccumulativeArbitrageProfit.Add(quoteProfit)
				s.Notify("%s grid arbitrage profit %v %s, accumulative arbitrage profit %v %s",
					s.Symbol,
					quoteProfit, s.Market.QuoteCurrency,
					s.state.AccumulativeArbitrageProfit, s.Market.QuoteCurrency,
				)
			}
		case types.SideTypeBuy:
			if sellOrder, ok := s.state.ArbitrageOrders[filledOrder.OrderID]; ok {
				// use base asset quantity here
				quoteProfit := sellOrder.Quantity.Mul(sellOrder.Price).
					Sub(filledOrder.Quantity.Mul(filledOrder.Price))
				s.state.AccumulativeArbitrageProfit = s.state.AccumulativeArbitrageProfit.Add(quoteProfit)
				s.Notify("%s grid arbitrage profit %v %s, accumulative arbitrage profit %v %s", s.Symbol,
					quoteProfit, s.Market.QuoteCurrency,
					s.state.AccumulativeArbitrageProfit, s.Market.QuoteCurrency,
				)
			}
		}
	}
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
				ArbitrageOrders: make(map[uint64]types.Order),
				Position:        types.NewPositionFromMarket(s.Market),
			}
		} else {
			s.state = &state
		}
	}

	// init profit stats
	s.state.ProfitStats.Init(s.Market)

	// field guards
	if s.state.ArbitrageOrders == nil {
		s.state.ArbitrageOrders = make(map[uint64]types.Order)
	}
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
		submitOrders := s.activeOrders.Backup()
		s.state.Orders = submitOrders

		if err := s.Persistence.Save(s.state, ID, instanceID); err != nil {
			return err
		}
	}
	return nil
}

// InstanceID returns the instance identifier from the current grid configuration parameters
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s-%d-%d-%d", ID, s.Symbol, s.GridNum, s.UpperPrice.Int(), s.LowerPrice.Int())
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// do some basic validation
	if s.GridNum == 0 {
		s.GridNum = 10
	}

	if s.Side == "" {
		s.Side = types.SideTypeBoth
	}

	instanceID := s.InstanceID()
	s.groupID = max.GenerateGroupID(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	if err := s.LoadState(); err != nil {
		return err
	}

	s.Notify("grid %s position", s.Symbol, s.state.Position)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	// we don't persist orders so that we can not clear the previous orders for now. just need time to support this.
	s.activeOrders = bbgo.NewLocalActiveOrderBook(s.Symbol)
	s.activeOrders.OnFilled(s.handleFilledOrder)
	s.activeOrders.BindStream(session.UserDataStream)

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.state.Position, s.orderStore)

	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		s.Notifiability.Notify(trade)
		s.state.ProfitStats.AddTrade(trade)
	})

	/*
		if s.TradeService != nil {
			s.tradeCollector.OnTrade(func(trade types.Trade) {
				if err := s.TradeService.Mark(ctx, trade.ID, ID); err != nil {
					log.WithError(err).Error("trade mark error")
				}
			})
		}
	*/

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		s.Notifiability.Notify(position)
	})
	s.tradeCollector.BindStream(session.UserDataStream)

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if err := s.SaveState(); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		} else {
			s.Notify("%s: %s grid is saved", ID, s.Symbol)
		}

		// now we can cancel the open orders
		log.Infof("canceling active orders...")
		if err := session.Exchange.CancelOrders(context.Background(), s.activeOrders.Orders()...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}
	})

	session.UserDataStream.OnStart(func() {
		// if we have orders in the state data, we can restore them
		if len(s.state.Orders) > 0 {
			s.Notifiability.Notify("restoring %s %d grid orders...", s.Symbol, len(s.state.Orders))

			createdOrders, err := orderExecutor.SubmitOrders(ctx, s.state.Orders...)
			if err != nil {
				log.WithError(err).Error("active orders restore error")
			}
			s.activeOrders.Add(createdOrders...)
			s.orderStore.Add(createdOrders...)
		} else {
			// or place new orders
			s.placeGridOrders(orderExecutor, session)
		}
	})

	if s.CatchUp {
		session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
			log.Infof("catchUp mode is enabled, updating grid orders...")
			// update grid
			s.placeGridOrders(orderExecutor, session)
		})
	}

	return nil
}
