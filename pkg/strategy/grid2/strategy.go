package grid2

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "grid2"

var log = logrus.WithField("strategy", ID)

var notionalModifier = fixedpoint.NewFromFloat(1.0001)

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment

	// Market stores the configuration of the market, for example, VolumePrecision, PricePrecision, MinLotSize... etc
	// This field will be injected automatically since we defined the Symbol field.
	types.Market `json:"-" yaml:"-"`

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol" yaml:"symbol"`

	// ProfitSpread is the fixed profit spread you want to submit the sell order
	ProfitSpread fixedpoint.Value `json:"profitSpread" yaml:"profitSpread"`

	// GridNum is the grid number, how many orders you want to post on the orderbook.
	GridNum int64 `json:"gridNumber" yaml:"gridNumber"`

	UpperPrice fixedpoint.Value `json:"upperPrice" yaml:"upperPrice"`

	LowerPrice fixedpoint.Value `json:"lowerPrice" yaml:"lowerPrice"`

	grid *Grid

	bbgo.QuantityOrAmount

	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	Position    *types.Position    `persistence:"position"`

	// orderStore is used to store all the created orders, so that we can filter the trades.
	orderStore *bbgo.OrderStore

	// activeOrders is the locally maintained active order book of the maker orders.
	activeOrders *bbgo.ActiveOrderBook

	tradeCollector *bbgo.TradeCollector

	orderExecutor *bbgo.GeneralOrderExecutor

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

	if s.GridNum == 0 {
		return fmt.Errorf("gridNum can not be zero")
	}

	if err := s.QuantityOrAmount.Validate(); err != nil {
		return err
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

// InstanceID returns the instance identifier from the current grid configuration parameters
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s-%d-%d-%d", ID, s.Symbol, s.GridNum, s.UpperPrice.Int(), s.LowerPrice.Int())
}

func (s *Strategy) handleOrderFilled(o types.Order) {

}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()

	s.groupID = util.FNV32(instanceID)

	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.Bind()
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})

	s.grid = NewGrid(s.LowerPrice, s.UpperPrice, fixedpoint.NewFromInt(s.GridNum), s.Market.TickSize)
	s.grid.CalculateArithmeticPins()

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		bbgo.Sync(ctx, s)

		// now we can cancel the open orders
		log.Infof("canceling active orders...")
		if err := session.Exchange.CancelOrders(context.Background(), s.activeOrders.Orders()...); err != nil {
			log.WithError(err).Errorf("cancel order error")
		}
	})

	session.UserDataStream.OnStart(func() {
		if err := s.setupGridOrders(ctx, session); err != nil {
			log.WithError(err).Errorf("failed to setup grid orders")
		}
	})

	return nil
}

func (s *Strategy) setupGridOrders(ctx context.Context, session *bbgo.ExchangeSession) error {
	lastPrice, err := s.getLastTradePrice(ctx, session)
	if err != nil {
		return errors.Wrap(err, "failed to get the last trade price")
	}

	// shift 1 grid because we will start from the buy order
	// if the buy order is filled, then we will submit another sell order at the higher grid.
	for i := len(s.grid.Pins) - 2; i >= 0; i++ {
		pin := s.grid.Pins[i]
		price := fixedpoint.Value(pin)

		if price.Compare(lastPrice) >= 0 {
			if s.QuantityOrAmount.Quantity.Sign() > 0 {
				quantity := s.QuantityOrAmount.Quantity

				createdOrders, err2 := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
					Symbol:      s.Symbol,
					Side:        types.SideTypeBuy,
					Type:        types.OrderTypeLimit,
					Quantity:    quantity,
					Price:       price,
					Market:      s.Market,
					TimeInForce: types.TimeInForceGTC,
					Tag:         "grid",
				})

				if err2 != nil {
					return err2
				}

				_ = createdOrders

			} else if s.QuantityOrAmount.Amount.Sign() > 0 {

			}
		}
	}

	return nil
}

func (s *Strategy) getLastTradePrice(ctx context.Context, session *bbgo.ExchangeSession) (fixedpoint.Value, error) {
	if bbgo.IsBackTesting {
		price, ok := session.LastPrice(s.Symbol)
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("last price of %s not found", s.Symbol)
		}

		return price, nil
	}

	tickers, err := session.Exchange.QueryTickers(ctx, s.Symbol)
	if err != nil {
		return fixedpoint.Zero, err
	}

	if ticker, ok := tickers[s.Symbol]; ok {
		if !ticker.Last.IsZero() {
			return ticker.Last, nil
		}

		// fallback to buy price
		return ticker.Buy, nil
	}

	return fixedpoint.Zero, fmt.Errorf("%s ticker price not found", s.Symbol)
}
