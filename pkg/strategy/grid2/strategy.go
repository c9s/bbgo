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

	tradeCollector *bbgo.TradeCollector

	orderExecutor *bbgo.GeneralOrderExecutor

	// groupID is the group ID used for the strategy instance for canceling orders
	groupID uint32

	logger *logrus.Entry
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

	if s.GridNum == 0 {
		return fmt.Errorf("gridNum can not be zero")
	}

	if err := s.QuantityOrAmount.Validate(); err != nil {
		if s.QuoteInvestment.IsZero() && s.BaseInvestment.IsZero() {
			return err
		}
	}

	if !s.QuantityOrAmount.IsSet() && s.QuoteInvestment.IsZero() && s.BaseInvestment.IsZero() {
		return fmt.Errorf("one of quantity, amount, quoteInvestment must be set")
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

	s.logger = log.WithFields(logrus.Fields{
		"symbol": s.Symbol,
	})

	s.groupID = util.FNV32(instanceID)

	s.logger.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

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
		s.logger.Infof("canceling active orders...")

		if err := s.orderExecutor.GracefulCancel(ctx); err != nil {
			log.WithError(err).Errorf("graceful order cancel error")
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

func (s *Strategy) checkRequiredInvestmentByQuantity(baseBalance, quoteBalance, quantity, lastPrice fixedpoint.Value, pins []Pin) (requiredBase, requiredQuote fixedpoint.Value, err error) {
	// check more investment budget details
	requiredBase = fixedpoint.Zero
	requiredQuote = fixedpoint.Zero

	// when we need to place a buy-to-sell conversion order, we need to mark the price
	buyPlacedPrice := fixedpoint.Zero
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		// TODO: add fee if we don't have the platform token. BNB, OKB or MAX...
		if price.Compare(lastPrice) >= 0 {
			// for orders that sell
			// if we still have the base balance
			if requiredBase.Add(quantity).Compare(baseBalance) <= 0 {
				requiredBase = requiredBase.Add(quantity)
			} else if i > 0 { // we do not want to sell at i == 0
				// convert sell to buy quote and add to requiredQuote
				nextLowerPin := pins[i-1]
				nextLowerPrice := fixedpoint.Value(nextLowerPin)
				requiredQuote = requiredQuote.Add(quantity.Mul(nextLowerPrice))
				buyPlacedPrice = nextLowerPrice
			}
		} else {
			// for orders that buy
			if !buyPlacedPrice.IsZero() && price.Compare(buyPlacedPrice) == 0 {
				continue
			}
			requiredQuote = requiredQuote.Add(quantity.Mul(price))
		}
	}

	if requiredBase.Compare(baseBalance) > 0 && requiredQuote.Compare(quoteBalance) > 0 {
		return requiredBase, requiredQuote, fmt.Errorf("both base balance (%f %s) or quote balance (%f %s) is not enough, required = base %f + quote %f",
			baseBalance.Float64(), s.Market.BaseCurrency,
			quoteBalance.Float64(), s.Market.QuoteCurrency,
			requiredBase.Float64(),
			requiredQuote.Float64())
	}

	if requiredBase.Compare(baseBalance) > 0 {
		return requiredBase, requiredQuote, fmt.Errorf("base balance (%f %s), required = base %f",
			baseBalance.Float64(), s.Market.BaseCurrency,
			requiredBase.Float64(),
		)
	}

	if requiredQuote.Compare(quoteBalance) > 0 {
		return requiredBase, requiredQuote, fmt.Errorf("quote balance (%f %s) is not enough, required = quote %f",
			quoteBalance.Float64(), s.Market.QuoteCurrency,
			requiredQuote.Float64(),
		)
	}

	return requiredBase, requiredQuote, nil
}

func (s *Strategy) checkRequiredInvestmentByAmount(baseBalance, quoteBalance, amount, lastPrice fixedpoint.Value, pins []Pin) (requiredBase, requiredQuote fixedpoint.Value, err error) {

	// check more investment budget details
	requiredBase = fixedpoint.Zero
	requiredQuote = fixedpoint.Zero

	// when we need to place a buy-to-sell conversion order, we need to mark the price
	buyPlacedPrice := fixedpoint.Zero
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		// TODO: add fee if we don't have the platform token. BNB, OKB or MAX...
		if price.Compare(lastPrice) >= 0 {
			// for orders that sell
			// if we still have the base balance
			quantity := amount.Div(lastPrice)
			if requiredBase.Add(quantity).Compare(baseBalance) <= 0 {
				requiredBase = requiredBase.Add(quantity)
			} else if i > 0 { // we do not want to sell at i == 0
				// convert sell to buy quote and add to requiredQuote
				nextLowerPin := pins[i-1]
				nextLowerPrice := fixedpoint.Value(nextLowerPin)
				requiredQuote = requiredQuote.Add(quantity.Mul(nextLowerPrice))
				buyPlacedPrice = nextLowerPrice
			}
		} else {
			// for orders that buy
			if !buyPlacedPrice.IsZero() && price.Compare(buyPlacedPrice) == 0 {
				continue
			}
			requiredQuote = requiredQuote.Add(amount)
		}
	}

	if requiredBase.Compare(baseBalance) > 0 && requiredQuote.Compare(quoteBalance) > 0 {
		return requiredBase, requiredQuote, fmt.Errorf("both base balance (%f %s) or quote balance (%f %s) is not enough, required = base %f + quote %f",
			baseBalance.Float64(), s.Market.BaseCurrency,
			quoteBalance.Float64(), s.Market.QuoteCurrency,
			requiredBase.Float64(),
			requiredQuote.Float64())
	}

	if requiredBase.Compare(baseBalance) > 0 {
		return requiredBase, requiredQuote, fmt.Errorf("base balance (%f %s), required = base %f",
			baseBalance.Float64(), s.Market.BaseCurrency,
			requiredBase.Float64(),
		)
	}

	if requiredQuote.Compare(quoteBalance) > 0 {
		return requiredBase, requiredQuote, fmt.Errorf("quote balance (%f %s) is not enough, required = quote %f",
			quoteBalance.Float64(), s.Market.QuoteCurrency,
			requiredQuote.Float64(),
		)
	}

	return requiredBase, requiredQuote, nil
}

func (s *Strategy) calculateQuoteInvestmentQuantity(quoteInvestment, lastPrice fixedpoint.Value, pins []Pin) (fixedpoint.Value, error) {
	buyPlacedPrice := fixedpoint.Zero

	// quoteInvestment = (p1 * q) + (p2 * q) + (p3 * q) + ....
	// =>
	// quoteInvestment = (p1 + p2 + p3) * q
	// q = quoteInvestment / (p1 + p2 + p3)
	totalQuotePrice := fixedpoint.Zero
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		if price.Compare(lastPrice) >= 0 {
			// for orders that sell
			// if we still have the base balance
			// quantity := amount.Div(lastPrice)
			if i > 0 { // we do not want to sell at i == 0
				// convert sell to buy quote and add to requiredQuote
				nextLowerPin := pins[i-1]
				nextLowerPrice := fixedpoint.Value(nextLowerPin)
				// requiredQuote = requiredQuote.Add(quantity.Mul(nextLowerPrice))
				totalQuotePrice = totalQuotePrice.Add(nextLowerPrice)
				buyPlacedPrice = nextLowerPrice
			}
		} else {
			// for orders that buy
			if !buyPlacedPrice.IsZero() && price.Compare(buyPlacedPrice) == 0 {
				continue
			}

			totalQuotePrice = totalQuotePrice.Add(price)
		}
	}

	return quoteInvestment.Div(totalQuotePrice), nil
}

func (s *Strategy) calculateQuoteBaseInvestmentQuantity(quoteInvestment, baseInvestment, lastPrice fixedpoint.Value, pins []Pin) (fixedpoint.Value, error) {
	s.logger.Infof("calculating quantity by quote/base investment: %f / %f", baseInvestment.Float64(), quoteInvestment.Float64())
	// q_p1 = q_p2 = q_p3 = q_p4
	// baseInvestment = q_p1 + q_p2 + q_p3 + q_p4 + ....
	// baseInvestment = numberOfSellOrders * q
	// maxBaseQuantity = baseInvestment / numberOfSellOrders
	// if maxBaseQuantity < minQuantity or maxBaseQuantity * priceLowest < minNotional
	// then reduce the numberOfSellOrders
	numberOfSellOrders := 0
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)
		if price.Compare(lastPrice) < 0 {
			break
		}
		numberOfSellOrders++
	}

	// if the maxBaseQuantity is less than minQuantity, then we need to reduce the number of the sell orders
	// so that the quantity can be increased.
	maxNumberOfSellOrders := numberOfSellOrders + 1
	minBaseQuantity := fixedpoint.Max(s.Market.MinNotional.Div(lastPrice), s.Market.MinQuantity)
	maxBaseQuantity := fixedpoint.Zero
	for maxBaseQuantity.Compare(s.Market.MinQuantity) <= 0 || maxBaseQuantity.Compare(minBaseQuantity) <= 0 {
		maxNumberOfSellOrders--
		maxBaseQuantity = baseInvestment.Div(fixedpoint.NewFromInt(int64(maxNumberOfSellOrders)))
	}
	s.logger.Infof("grid %s base investment sell orders: %d", s.Symbol, maxNumberOfSellOrders)
	if maxNumberOfSellOrders > 0 {
		s.logger.Infof("grid %s base investment quantity range: %f <=> %f", s.Symbol, minBaseQuantity.Float64(), maxBaseQuantity.Float64())
	}

	buyPlacedPrice := fixedpoint.Zero
	totalQuotePrice := fixedpoint.Zero
	// quoteInvestment = (p1 * q) + (p2 * q) + (p3 * q) + ....
	// =>
	// quoteInvestment = (p1 + p2 + p3) * q
	// maxBuyQuantity = quoteInvestment / (p1 + p2 + p3)
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		if price.Compare(lastPrice) >= 0 {
			// for orders that sell
			// if we still have the base balance
			// quantity := amount.Div(lastPrice)
			if i > 0 { // we do not want to sell at i == 0
				// convert sell to buy quote and add to requiredQuote
				nextLowerPin := pins[i-1]
				nextLowerPrice := fixedpoint.Value(nextLowerPin)
				// requiredQuote = requiredQuote.Add(quantity.Mul(nextLowerPrice))
				totalQuotePrice = totalQuotePrice.Add(nextLowerPrice)
				buyPlacedPrice = nextLowerPrice
			}
		} else {
			// for orders that buy
			if !buyPlacedPrice.IsZero() && price.Compare(buyPlacedPrice) == 0 {
				continue
			}

			totalQuotePrice = totalQuotePrice.Add(price)
		}
	}

	quoteSideQuantity := quoteInvestment.Div(totalQuotePrice)
	if maxNumberOfSellOrders > 0 {
		return fixedpoint.Max(quoteSideQuantity, maxBaseQuantity), nil
	}

	return quoteSideQuantity, nil
}

// setupGridOrders
// 1) if quantity or amount is set, we should use quantity/amount directly instead of using investment amount to calculate.
// 2) if baseInvestment, quoteInvestment is set, then we should calculate the quantity from the given base investment and quote investment.
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
	if s.QuantityOrAmount.IsSet() {
		if quantity := s.QuantityOrAmount.Quantity; !quantity.IsZero() {
			if _, _, err2 := s.checkRequiredInvestmentByQuantity(totalBase, totalQuote, lastPrice, s.QuantityOrAmount.Quantity, s.grid.Pins); err != nil {
				return err2
			}
		}
		if amount := s.QuantityOrAmount.Amount; !amount.IsZero() {
			if _, _, err2 := s.checkRequiredInvestmentByAmount(totalBase, totalQuote, lastPrice, amount, s.grid.Pins); err != nil {
				return err2
			}
		}
	} else {
		// calculate the quantity from the investment configuration
		if !s.QuoteInvestment.IsZero() && !s.BaseInvestment.IsZero() {
			quantity, err2 := s.calculateQuoteBaseInvestmentQuantity(s.QuoteInvestment, s.BaseInvestment, lastPrice, s.grid.Pins)
			if err2 != nil {
				return err2
			}
			s.QuantityOrAmount.Quantity = quantity

		} else if !s.QuoteInvestment.IsZero() {
			quantity, err2 := s.calculateQuoteInvestmentQuantity(s.QuoteInvestment, lastPrice, s.grid.Pins)
			if err2 != nil {
				return err2
			}
			s.QuantityOrAmount.Quantity = quantity
		}
	}

	// if base investment and quote investment is set, when we should check if the
	// investment configuration is valid with the current balances
	if !s.BaseInvestment.IsZero() && !s.QuoteInvestment.IsZero() {
		if s.BaseInvestment.Compare(totalBase) > 0 {
			return fmt.Errorf("baseInvestment setup %f is greater than the total base balance %f", s.BaseInvestment.Float64(), totalBase.Float64())
		}
		if s.QuoteInvestment.Compare(totalQuote) > 0 {
			return fmt.Errorf("quoteInvestment setup %f is greater than the total quote balance %f", s.QuoteInvestment.Float64(), totalQuote.Float64())
		}

		if !s.QuantityOrAmount.IsSet() {
			// TODO: calculate and override the quantity here
		}
	}

	var buyPlacedPrice = fixedpoint.Zero
	var pins = s.grid.Pins
	var usedBase = fixedpoint.Zero
	var usedQuote = fixedpoint.Zero
	var submitOrders []types.SubmitOrder
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)
		quantity := s.QuantityOrAmount.Quantity
		if quantity.IsZero() {
			quantity = s.QuantityOrAmount.Amount.Div(price)
		}

		// TODO: add fee if we don't have the platform token. BNB, OKB or MAX...
		if price.Compare(lastPrice) >= 0 {
			if usedBase.Add(quantity).Compare(totalBase) < 0 {
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Type:        types.OrderTypeLimitMaker,
					Side:        types.SideTypeSell,
					Price:       price,
					Quantity:    quantity,
					Market:      s.Market,
					TimeInForce: types.TimeInForceGTC,
					Tag:         "grid",
				})
				usedBase = usedBase.Add(quantity)
			} else if i > 0 {
				// next price
				nextPin := pins[i-1]
				nextPrice := fixedpoint.Value(nextPin)
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Type:        types.OrderTypeLimitMaker,
					Side:        types.SideTypeBuy,
					Price:       nextPrice,
					Quantity:    quantity,
					Market:      s.Market,
					TimeInForce: types.TimeInForceGTC,
					Tag:         "grid",
				})
				quoteQuantity := quantity.Mul(price)
				usedQuote = usedQuote.Add(quoteQuantity)
				buyPlacedPrice = nextPrice
			}
		} else {
			if !buyPlacedPrice.IsZero() && price.Compare(buyPlacedPrice) >= 0 {
				continue
			}

			submitOrders = append(submitOrders, types.SubmitOrder{
				Symbol:      s.Symbol,
				Type:        types.OrderTypeLimitMaker,
				Side:        types.SideTypeBuy,
				Price:       price,
				Quantity:    quantity,
				Market:      s.Market,
				TimeInForce: types.TimeInForceGTC,
				Tag:         "grid",
			})
			quoteQuantity := quantity.Mul(price)
			usedQuote = usedQuote.Add(quoteQuantity)
		}

		createdOrders, err2 := s.orderExecutor.SubmitOrders(ctx, submitOrders...)
		if err2 != nil {
			return err
		}
		for _, order := range createdOrders {
			s.logger.Infof(order.String())
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
