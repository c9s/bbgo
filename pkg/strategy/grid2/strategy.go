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

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type GridProfitStats struct {
	TotalProfit    fixedpoint.Value `json:"totalProfit"`
	FloatProfit    fixedpoint.Value `json:"floatProfit"`
	GridProfit     fixedpoint.Value `json:"gridProfit"`
	ArbitrageCount int              `json:"arbitrageCount"`
	TotalFee       fixedpoint.Value `json:"totalFee"`
	Volume         fixedpoint.Value `json:"volume"`
}

type Strategy struct {
	Environment *bbgo.Environment

	// Market stores the configuration of the market, for example, VolumePrecision, PricePrecision, MinLotSize... etc
	// This field will be injected automatically since we defined the Symbol field.
	types.Market `json:"-"`

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol"`

	// ProfitSpread is the fixed profit spread you want to submit the sell order
	ProfitSpread fixedpoint.Value `json:"profitSpread"`

	// GridNum is the grid number, how many orders you want to post on the orderbook.
	GridNum int64 `json:"gridNumber"`

	UpperPrice fixedpoint.Value `json:"upperPrice"`

	LowerPrice fixedpoint.Value `json:"lowerPrice"`

	// QuantityOrAmount embeds the Quantity field and the Amount field
	// If you set up the Quantity field or the Amount field, you don't need to set the QuoteInvestment and BaseInvestment
	bbgo.QuantityOrAmount

	// If Quantity and Amount is not set, we can use the quote investment to calculate our quantity.
	QuoteInvestment fixedpoint.Value `json:"quoteInvestment"`

	// BaseInvestment is the total base quantity you want to place as the sell order.
	BaseInvestment fixedpoint.Value `json:"baseInvestment"`

	grid *Grid

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

type InvestmentBudget struct {
	baseInvestment  fixedpoint.Value
	quoteInvestment fixedpoint.Value
	baseBalance     fixedpoint.Value
	quoteBalance    fixedpoint.Value
}

func (s *Strategy) checkRequiredInvestmentByQuantity(baseInvestment, quoteInvestment, baseBalance, quoteBalance, quantity, lastPrice fixedpoint.Value, pins []Pin) error {
	if baseInvestment.Compare(baseBalance) > 0 {
		return fmt.Errorf("baseInvestment setup %f is greater than the total base balance %f", baseInvestment.Float64(), baseBalance.Float64())
	}

	if quoteInvestment.Compare(quoteBalance) > 0 {
		return fmt.Errorf("quoteInvestment setup %f is greater than the total quote balance %f", quoteInvestment.Float64(), quoteBalance.Float64())
	}

	// check more investment budget details
	requiredBase := fixedpoint.Zero
	requiredQuote := fixedpoint.Zero
	for i := len(pins) - 1; i >= 0; i++ {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		// TODO: add fee if we don't have the platform token. BNB, OKB or MAX...
		if price.Compare(lastPrice) >= 0 {
			// for orders that sell
			// if we still have the base balance
			if requiredBase.Compare(baseBalance) < 0 {
				requiredBase = requiredBase.Add(quantity)
			} else if i > 0 { // we do not want to sell at i == 0
				// convert sell to buy quote and add to requiredQuote
				nextLowerPin := s.grid.Pins[i-1]
				nextLowerPrice := fixedpoint.Value(nextLowerPin)
				requiredQuote = requiredQuote.Add(quantity.Mul(nextLowerPrice))
			}
		} else {
			// for orders that buy
			requiredQuote = requiredQuote.Add(quantity.Mul(price))
		}
	}

	if requiredBase.Compare(baseBalance) > 0 && requiredQuote.Compare(quoteBalance) > 0 {
		return fmt.Errorf("both base balance (%f %s) and quote balance (%f %s) are not enought",
			baseBalance.Float64(), s.Market.BaseCurrency,
			quoteBalance.Float64(), s.Market.QuoteCurrency)
	}

	if requiredBase.Compare(baseBalance) < 0 {
		// see if we can convert some quotes to base
	}

	return nil
}

func (s *Strategy) setupGridOrders(ctx context.Context, session *bbgo.ExchangeSession) error {
	lastPrice, err := s.getLastTradePrice(ctx, session)
	if err != nil {
		return errors.Wrap(err, "failed to get the last trade price")
	}

	// check if base and quote are enough
	baseBalance, ok := session.Account.Balance(s.Market.BaseCurrency)
	if !ok {
		return fmt.Errorf("base %s balance not found", s.Market.BaseCurrency)
	}

	quoteBalance, ok := session.Account.Balance(s.Market.QuoteCurrency)
	if !ok {
		return fmt.Errorf("quote %s balance not found", s.Market.QuoteCurrency)
	}

	totalBase := baseBalance.Available
	totalQuote := quoteBalance.Available

	// shift 1 grid because we will start from the buy order
	// if the buy order is filled, then we will submit another sell order at the higher grid.
	quantityOrAmountIsSet := s.QuantityOrAmount.IsSet()
	if quantityOrAmountIsSet {
		if err2 := s.checkRequiredInvestmentByQuantity(
			s.BaseInvestment, s.QuoteInvestment,
			totalBase, totalQuote,
			lastPrice, s.QuantityOrAmount.Quantity, s.grid.Pins); err != nil {
			return err2
		}
	}

	for i := len(s.grid.Pins) - 2; i >= 0; i++ {
		pin := s.grid.Pins[i]
		price := fixedpoint.Value(pin)

		if price.Compare(lastPrice) >= 0 {
			// check sell order
			if quantityOrAmountIsSet {
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
			} else if s.BaseInvestment.Sign() > 0 {

			} else {
				// error: either quantity, amount, baseInvestment is not set.
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
