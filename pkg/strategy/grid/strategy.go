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
	Position        *bbgo.Position                `json:"position,omitempty"`

	AccumulativeArbitrageProfit fixedpoint.Value `json:"accumulativeArbitrageProfit"`

	// any created orders for tracking trades
	// [source Order ID] -> arbitrage order
	ArbitrageOrders map[uint64]types.Order `json:"arbitrageOrders"`
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
	GridNum int `json:"gridNumber" yaml:"gridNumber"`

	UpperPrice fixedpoint.Value `json:"upperPrice" yaml:"upperPrice"`

	LowerPrice fixedpoint.Value `json:"lowerPrice" yaml:"lowerPrice"`

	// Quantity is the quantity you want to submit for each order.
	Quantity fixedpoint.Value `json:"quantity,omitempty"`

	// ScaleQuantity helps user to define the quantity by price scale or volume scale
	ScaleQuantity *bbgo.PriceVolumeScale `json:"scaleQuantity,omitempty"`

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

	// groupID is the group ID used for the strategy instance for canceling orders
	groupID uint32
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
	if currentPrice > s.UpperPrice {
		return nil, fmt.Errorf("current price %f is higher than upper price %f", currentPrice.Float64(), s.UpperPrice.Float64())
	}

	priceRange := s.UpperPrice - s.LowerPrice
	numGrids := fixedpoint.NewFromInt(s.GridNum)
	gridSpread := priceRange.Div(numGrids)

	// find the nearest grid price from the current price
	startPrice := fixedpoint.Max(
		s.LowerPrice,
		s.UpperPrice-(s.UpperPrice-currentPrice).Div(gridSpread).Floor().Mul(gridSpread))

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

		if _, filled := s.state.FilledSellGrids[price]; filled {
			log.Debugf("sell grid at price %f is already filled, skipping", price.Float64())
			continue
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

		s.state.FilledSellGrids[price] = struct{}{}
	}

	return orders, nil
}

func (s *Strategy) generateGridBuyOrders(session *bbgo.ExchangeSession) ([]types.SubmitOrder, error) {
	// session.Exchange.QueryTicker()
	currentPriceFloat, ok := session.LastPrice(s.Symbol)
	if !ok {
		return nil, fmt.Errorf("%s last price not found, skipping", s.Symbol)
	}

	currentPrice := fixedpoint.NewFromFloat(currentPriceFloat)
	if currentPrice < s.LowerPrice {
		return nil, fmt.Errorf("current price %f is lower than the lower price %f", currentPrice.Float64(), s.LowerPrice.Float64())
	}

	priceRange := s.UpperPrice - s.LowerPrice
	numGrids := fixedpoint.NewFromInt(s.GridNum)
	gridSpread := priceRange.Div(numGrids)

	// Find the nearest grid price for placing buy orders:
	// buyRange = currentPrice - lowerPrice
	// numOfBuyGrids = Floor(buyRange / gridSpread)
	// startPrice = lowerPrice + numOfBuyGrids * gridSpread
	// priceOfBuyOrder1 = startPrice
	// priceOfBuyOrder2 = startPrice - gridSpread
	// priceOfBuyOrder3 = startPrice - gridSpread * 2
	startPrice := fixedpoint.Min(
		s.UpperPrice,
		s.LowerPrice+(currentPrice-s.LowerPrice).Div(gridSpread).Floor().Mul(gridSpread))

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
		startPrice.Float64(),
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

		if _, filled := s.state.FilledBuyGrids[price]; filled {
			log.Debugf("buy grid at price %f is already filled, skipping", price.Float64())
			continue
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

		s.state.FilledBuyGrids[price] = struct{}{}
	}

	return orders, nil
}

func (s *Strategy) placeGridSellOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	orderForms, err := s.generateGridSellOrders(session)
	if err != nil {
		return err
	}

	if len(orderForms) > 0 {
		createdOrders, err := orderExecutor.SubmitOrders(context.Background(), orderForms...)
		if err != nil {
			return err
		}

		s.activeOrders.Add(createdOrders...)
	}

	return nil
}

func (s *Strategy) placeGridBuyOrders(orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	orderForms, err := s.generateGridBuyOrders(session)
	if err != nil {
		return err
	}

	if len(orderForms) > 0 {
		createdOrders, err := orderExecutor.SubmitOrders(context.Background(), orderForms...)
		if err != nil {
			return err
		} else {
			s.activeOrders.Add(createdOrders...)
		}
	}

	return nil
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
	}
}

func (s *Strategy) tradeUpdateHandler(trade types.Trade) {
	if trade.Symbol != s.Symbol {
		return
	}

	if s.orderStore.Exists(trade.OrderID) {
		log.Infof("received trade update of order %d: %+v", trade.OrderID, trade)

		if s.TradeService != nil {
			if err := s.TradeService.Mark(context.Background(), trade.ID, ID); err != nil {
				log.WithError(err).Error("trade mark error")
			}
		}

		if trade.Side == types.SideTypeSelf {
			return
		}

		profit, madeProfit := s.state.Position.AddTrade(trade)
		if madeProfit {
			s.Notify("average cost profit: %f", profit.Float64())
		}
	}
}

func (s *Strategy) handleFilledOrder(filledOrder types.Order) {
	// generate arbitrage order
	var side = filledOrder.Side.Reverse()
	var price = filledOrder.Price
	var quantity = filledOrder.Quantity

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
		var amount = filledOrder.Price * filledOrder.Quantity
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

	log.Infof("submitting arbitrage order: %s against filled order %s", submitOrder.String(), filledOrder.String())

	createdOrders, err := s.OrderExecutor.SubmitOrders(context.Background(), submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place orders")
		return
	}

	// create one-way link from the newly created orders
	for _, o := range createdOrders {
		s.state.ArbitrageOrders[o.OrderID] = filledOrder
	}

	s.orderStore.Add(createdOrders...)
	s.activeOrders.Add(createdOrders...)

	// calculate arbitrage profit
	// TODO: apply fee rate here
	if s.Long {
		switch filledOrder.Side {
		case types.SideTypeSell:
			if buyOrder, ok := s.state.ArbitrageOrders[filledOrder.OrderID]; ok {
				// use base asset quantity here
				baseProfit := buyOrder.Quantity - filledOrder.Quantity
				s.state.AccumulativeArbitrageProfit += fixedpoint.NewFromFloat(baseProfit)
				s.Notify("%s grid arbitrage profit %f %s, accumulative arbitrage profit %f %s", s.Symbol,
					baseProfit, s.Market.BaseCurrency,
					s.state.AccumulativeArbitrageProfit.Float64(), s.Market.BaseCurrency,
				)
			}

		case types.SideTypeBuy:
			if sellOrder, ok := s.state.ArbitrageOrders[filledOrder.OrderID]; ok {
				// use base asset quantity here
				baseProfit := filledOrder.Quantity - sellOrder.Quantity
				s.state.AccumulativeArbitrageProfit += fixedpoint.NewFromFloat(baseProfit)
				s.Notify("%s grid arbitrage profit %f %s, accumulative arbitrage profit %f %s", s.Symbol,
					baseProfit, s.Market.BaseCurrency,
					s.state.AccumulativeArbitrageProfit.Float64(), s.Market.BaseCurrency,
				)
			}
		}
	} else if !s.Long && s.Quantity > 0 {
		switch filledOrder.Side {
		case types.SideTypeSell:
			if buyOrder, ok := s.state.ArbitrageOrders[filledOrder.OrderID]; ok {
				// use base asset quantity here
				quoteProfit := (filledOrder.Quantity * filledOrder.Price) - (buyOrder.Quantity * buyOrder.Price)
				s.state.AccumulativeArbitrageProfit += fixedpoint.NewFromFloat(quoteProfit)
				s.Notify("%s grid arbitrage profit %f %s, accumulative arbitrage profit %f %s", s.Symbol,
					quoteProfit, s.Market.QuoteCurrency,
					s.state.AccumulativeArbitrageProfit.Float64(), s.Market.QuoteCurrency,
				)
			}
		case types.SideTypeBuy:
			if sellOrder, ok := s.state.ArbitrageOrders[filledOrder.OrderID]; ok {
				// use base asset quantity here
				quoteProfit := (sellOrder.Quantity * sellOrder.Price) - (filledOrder.Quantity * filledOrder.Price)
				s.state.AccumulativeArbitrageProfit += fixedpoint.NewFromFloat(quoteProfit)
				s.Notify("%s grid arbitrage profit %f %s, accumulative arbitrage profit %f %s", s.Symbol,
					quoteProfit, s.Market.QuoteCurrency,
					s.state.AccumulativeArbitrageProfit.Float64(), s.Market.QuoteCurrency,
				)
			}
		}
	}
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// do some basic validation
	if s.GridNum == 0 {
		s.GridNum = 10
	}

	if s.Side == "" {
		s.Side = types.SideTypeBoth
	}

	if s.UpperPrice == 0 {
		return errors.New("upperPrice can not be zero, you forgot to set?")
	}

	if s.LowerPrice == 0 {
		return errors.New("lowerPrice can not be zero, you forgot to set?")
	}

	if s.UpperPrice <= s.LowerPrice {
		return fmt.Errorf("upperPrice (%f) should not be less than or equal to lowerPrice (%f)", s.UpperPrice.Float64(), s.LowerPrice.Float64())
	}

	instanceID := fmt.Sprintf("grid-%s-%d-%d-%d", s.Symbol, s.GridNum, s.UpperPrice, s.LowerPrice)
	s.groupID = max.GenerateGroupID(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	var stateLoaded = false
	if s.Persistence != nil {
		var state State
		if err := s.Persistence.Load(&state, ID, instanceID); err != nil {
			if err != service.ErrPersistenceNotExists {
				return errors.Wrapf(err, "state load error")
			}
		} else {
			log.Infof("grid state loaded")
			stateLoaded = true
			s.state = &state
		}
	}

	if s.state == nil {
		position, ok := session.Position(s.Symbol)
		if !ok {
			return fmt.Errorf("position not found")
		}

		s.state = &State{
			FilledBuyGrids:  make(map[fixedpoint.Value]struct{}),
			FilledSellGrids: make(map[fixedpoint.Value]struct{}),
			ArbitrageOrders: make(map[uint64]types.Order),
			Position:        position,
		}
	}

	if s.state.ArbitrageOrders == nil {
		s.state.ArbitrageOrders = make(map[uint64]types.Order)
	}

	s.Notify("current position %+v", s.state.Position)

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.Stream)

	// we don't persist orders so that we can not clear the previous orders for now. just need time to support this.
	s.activeOrders = bbgo.NewLocalActiveOrderBook()
	s.activeOrders.OnFilled(s.handleFilledOrder)
	s.activeOrders.BindStream(session.Stream)

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if s.Persistence != nil {
			log.Infof("backing up grid state...")
			submitOrders := s.activeOrders.Backup()
			s.state.Orders = submitOrders
			if err := s.Persistence.Save(s.state, ID, instanceID); err != nil {
				log.WithError(err).Error("can not save active order backups")
			} else {
				log.Infof("active order snapshot saved")
			}
		}

		log.Infof("canceling active orders...")
		if err := session.Exchange.CancelOrders(ctx, s.activeOrders.Orders()...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}
	})

	if s.CatchUp {
		session.Stream.OnKLineClosed(func(kline types.KLine) {
			log.Infof("catchUp mode is enabled, updating grid orders...")
			// update grid
			s.placeGridOrders(orderExecutor, session)
		})
	}

	session.Stream.OnTradeUpdate(s.tradeUpdateHandler)
	session.Stream.OnStart(func() {
		if stateLoaded && len(s.state.Orders) > 0 {
			createdOrders, err := orderExecutor.SubmitOrders(ctx, s.state.Orders...)
			if err != nil {
				log.WithError(err).Error("active orders restore error")
			}
			s.activeOrders.Add(createdOrders...)
			s.orderStore.Add(createdOrders...)
		} else {
			s.placeGridOrders(orderExecutor, session)
		}
	})

	return nil
}
