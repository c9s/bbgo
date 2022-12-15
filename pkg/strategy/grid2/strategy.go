package grid2

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "grid2"

const orderTag = "grid2"

var log = logrus.WithField("strategy", ID)

var maxNumberOfOrderTradesQueryTries = 10

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

//go:generate mockgen -destination=mocks/order_executor.go -package=mocks . OrderExecutor
type OrderExecutor interface {
	SubmitOrders(ctx context.Context, submitOrders ...types.SubmitOrder) (types.OrderSlice, error)
	ClosePosition(ctx context.Context, percentage fixedpoint.Value, tags ...string) error
	GracefulCancel(ctx context.Context, orders ...types.Order) error
}

type Strategy struct {
	Environment *bbgo.Environment

	// Market stores the configuration of the market, for example, VolumePrecision, PricePrecision, MinLotSize... etc
	// This field will be injected automatically since we defined the Symbol field.
	types.Market `json:"-"`

	// These fields will be filled from the config file (it translates YAML to JSON)
	Symbol string `json:"symbol"`

	// ProfitSpread is the fixed profit spread you want to submit the sell order
	// When ProfitSpread is enabled, the grid will shift up, e.g.,
	// If you opened a grid with the price range 10_000 to 20_000
	// With profit spread set to 3_000
	// The sell orders will be placed in the range 13_000 to 23_000
	// And the buy orders will be placed in the original price range 10_000 to 20_000
	ProfitSpread fixedpoint.Value `json:"profitSpread"`

	// GridNum is the grid number, how many orders you want to post on the orderbook.
	GridNum int64 `json:"gridNumber"`

	UpperPrice fixedpoint.Value `json:"upperPrice"`

	LowerPrice fixedpoint.Value `json:"lowerPrice"`

	// Compound option is used for buying more inventory when
	// the profit is made by the filled sell order.
	Compound bool `json:"compound"`

	// EarnBase option is used for earning profit in base currency.
	// e.g. earn BTC in BTCUSDT and earn ETH in ETHUSDT
	// instead of earn USDT in BTCUSD
	EarnBase bool `json:"earnBase"`

	// QuantityOrAmount embeds the Quantity field and the Amount field
	// If you set up the Quantity field or the Amount field, you don't need to set the QuoteInvestment and BaseInvestment
	bbgo.QuantityOrAmount

	// If Quantity and Amount is not set, we can use the quote investment to calculate our quantity.
	QuoteInvestment fixedpoint.Value `json:"quoteInvestment"`

	// BaseInvestment is the total base quantity you want to place as the sell order.
	BaseInvestment fixedpoint.Value `json:"baseInvestment"`

	TriggerPrice fixedpoint.Value `json:"triggerPrice"`

	StopLossPrice   fixedpoint.Value `json:"stopLossPrice"`
	TakeProfitPrice fixedpoint.Value `json:"takeProfitPrice"`

	// CloseWhenCancelOrder option is used to close the grid if any of the order is canceled.
	// This option let you simply remote control the grid from the crypto exchange mobile app.
	CloseWhenCancelOrder bool `json:"closeWhenCancelOrder"`

	// KeepOrdersWhenShutdown option is used for keeping the grid orders when shutting down bbgo
	KeepOrdersWhenShutdown bool `json:"keepOrdersWhenShutdown"`

	// ClearOpenOrdersWhenStart
	// If this is set, when bbgo started, it will clear the open orders in the same market (by symbol)
	ClearOpenOrdersWhenStart bool `json:"clearOpenOrdersWhenStart"`

	ResetPositionWhenStart bool `json:"resetPositionWhenStart"`

	// FeeRate is used for calculating the minimal profit spread.
	// it makes sure that your grid configuration is profitable.
	FeeRate fixedpoint.Value `json:"feeRate"`

	SkipSpreadCheck bool `json:"skipSpreadCheck"`

	GridProfitStats *GridProfitStats   `persistence:"grid_profit_stats"`
	ProfitStats     *types.ProfitStats `persistence:"profit_stats"`
	Position        *types.Position    `persistence:"position"`

	grid              *Grid
	session           *bbgo.ExchangeSession
	orderQueryService types.ExchangeOrderQueryService

	orderExecutor    OrderExecutor
	historicalTrades *bbgo.TradeStore

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

	if !s.SkipSpreadCheck {
		if err := s.checkSpread(); err != nil {
			return errors.Wrapf(err, "spread is too small, please try to reduce your gridNum or increase the price range (upperPrice and lowerPrice)")
		}
	}

	if !s.QuantityOrAmount.IsSet() && s.QuoteInvestment.IsZero() {
		return fmt.Errorf("either quantity, amount or quoteInvestment must be set")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
}

// InstanceID returns the instance identifier from the current grid configuration parameters
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s-%d-%d-%d", ID, s.Symbol, s.GridNum, s.UpperPrice.Int(), s.LowerPrice.Int())
}

func (s *Strategy) checkSpread() error {
	gridNum := fixedpoint.NewFromInt(s.GridNum)
	spread := s.ProfitSpread
	if spread.IsZero() {
		spread = s.UpperPrice.Sub(s.LowerPrice).Div(gridNum)
	}

	feeRate := s.FeeRate
	if feeRate.IsZero() {
		feeRate = fixedpoint.NewFromFloat(0.075 * 0.01)
	}

	// the min fee rate from 2 maker/taker orders (with 0.1 rate for profit)
	gridFeeRate := feeRate.Mul(fixedpoint.NewFromFloat(2.01))

	if spread.Div(s.LowerPrice).Compare(gridFeeRate) < 0 {
		return fmt.Errorf("profitSpread %f %s is too small for lower price, less than the grid fee rate: %s", spread.Float64(), spread.Div(s.LowerPrice).Percentage(), gridFeeRate.Percentage())
	}

	if spread.Div(s.UpperPrice).Compare(gridFeeRate) < 0 {
		return fmt.Errorf("profitSpread %f %s is too small for upper price, less than the grid fee rate: %s", spread.Float64(), spread.Div(s.UpperPrice).Percentage(), gridFeeRate.Percentage())
	}

	return nil
}

func (s *Strategy) handleOrderCanceled(o types.Order) {
	s.logger.Infof("GRID ORDER CANCELED: %s", o.String())

	ctx := context.Background()
	if s.CloseWhenCancelOrder {
		s.logger.Infof("one of the grid orders is canceled, now closing grid...")
		if err := s.closeGrid(ctx); err != nil {
			s.logger.WithError(err).Errorf("graceful order cancel error")
		}
	}
}

func (s *Strategy) calculateProfit(o types.Order, buyPrice, buyQuantity fixedpoint.Value) *GridProfit {
	if s.EarnBase {
		// sell quantity - buy quantity
		profitQuantity := o.Quantity.Sub(buyQuantity)
		profit := &GridProfit{
			Currency: s.Market.BaseCurrency,
			Profit:   profitQuantity,
			Time:     o.UpdateTime.Time(),
			Order:    o,
		}
		return profit
	}

	// earn quote
	// (sell_price - buy_price) * quantity
	profitQuantity := o.Price.Sub(buyPrice).Mul(o.Quantity)
	profit := &GridProfit{
		Currency: s.Market.QuoteCurrency,
		Profit:   profitQuantity,
		Time:     o.UpdateTime.Time(),
		Order:    o,
	}
	return profit
}

// collectTradeFee collects the fee from the given trade slice
func collectTradeFee(trades []types.Trade) map[string]fixedpoint.Value {
	fees := make(map[string]fixedpoint.Value)
	for _, t := range trades {
		if fee, ok := fees[t.FeeCurrency]; ok {
			fees[t.FeeCurrency] = fee.Add(t.Fee)
		} else {
			fees[t.FeeCurrency] = t.Fee
		}
	}
	return fees
}

func aggregateTradesQuantity(trades []types.Trade) fixedpoint.Value {
	tq := fixedpoint.Zero
	for _, t := range trades {
		tq = tq.Add(t.Quantity)
	}
	return tq
}

func (s *Strategy) verifyOrderTrades(o types.Order, trades []types.Trade) bool {
	tq := aggregateTradesQuantity(trades)

	if tq.Compare(o.Quantity) != 0 {
		s.logger.Warnf("order trades missing. expected: %f actual: %f",
			o.Quantity.Float64(),
			tq.Float64())
		return false
	}

	return true
}

// aggregateOrderBaseFee collects the base fee quantity from the given order
// it falls back to query the trades via the RESTful API when the websocket trades are not all received.
func (s *Strategy) aggregateOrderBaseFee(o types.Order) fixedpoint.Value {
	// try to get the received trades (websocket trades)
	orderTrades := s.historicalTrades.GetOrderTrades(o)
	if len(orderTrades) > 0 {
		s.logger.Infof("found filled order trades: %+v", orderTrades)
	}

	for maxTries := maxNumberOfOrderTradesQueryTries; maxTries > 0; maxTries-- {
		// if one of the trades is missing, we need to query the trades from the RESTful API
		if s.verifyOrderTrades(o, orderTrades) {
			// if trades are verified
			fees := collectTradeFee(orderTrades)
			if fee, ok := fees[s.Market.BaseCurrency]; ok {
				return fee
			}
			return fixedpoint.Zero
		}

		// if we don't support orderQueryService, then we should just skip
		if s.orderQueryService == nil {
			return fixedpoint.Zero
		}

		s.logger.Warnf("missing order trades or missing trade fee, pulling order trades from API")

		// if orderQueryService is supported, use it to query the trades of the filled order
		apiOrderTrades, err := s.orderQueryService.QueryOrderTrades(context.Background(), types.OrderQuery{
			Symbol:  o.Symbol,
			OrderID: strconv.FormatUint(o.OrderID, 10),
		})
		if err != nil {
			s.logger.WithError(err).Errorf("query order trades error")
		} else {
			s.logger.Infof("fetched api trades: %+v", apiOrderTrades)
			orderTrades = apiOrderTrades
		}
	}

	return fixedpoint.Zero
}

func (s *Strategy) processFilledOrder(o types.Order) {
	// check order fee
	newSide := types.SideTypeSell
	newPrice := o.Price
	newQuantity := o.Quantity
	orderQuoteQuantity := o.Quantity.Mul(o.Price)

	// collect trades
	baseSellQuantityReduction := fixedpoint.Zero

	// baseSellQuantityReduction calculation should be only for BUY order
	// because when 1.0 BTC buy order is filled without FEE token, then we will actually get 1.0 * (1 - feeRate) BTC
	// if we don't reduce the sell quantity, than we might fail to place the sell order
	if o.Side == types.SideTypeBuy {
		baseSellQuantityReduction = s.aggregateOrderBaseFee(o)

		s.logger.Infof("buy order base fee: %f %s", baseSellQuantityReduction.Float64(), s.Market.BaseCurrency)

		newQuantity = newQuantity.Sub(baseSellQuantityReduction)
	}

	switch o.Side {
	case types.SideTypeSell:
		newSide = types.SideTypeBuy

		if !s.ProfitSpread.IsZero() {
			newPrice = newPrice.Sub(s.ProfitSpread)
		} else {
			if pin, ok := s.grid.NextLowerPin(newPrice); ok {
				newPrice = fixedpoint.Value(pin)
			}
		}

		// use the profit to buy more inventory in the grid
		if s.Compound || s.EarnBase {
			newQuantity = fixedpoint.Max(orderQuoteQuantity.Div(newPrice), s.Market.MinQuantity)
		}

		// calculate profit
		// TODO: send profit notification
		profit := s.calculateProfit(o, newPrice, newQuantity)
		s.logger.Infof("GENERATED GRID PROFIT: %+v", profit)
		s.GridProfitStats.AddProfit(profit)

	case types.SideTypeBuy:
		newSide = types.SideTypeSell
		if !s.ProfitSpread.IsZero() {
			newPrice = newPrice.Add(s.ProfitSpread)
		} else {
			if pin, ok := s.grid.NextHigherPin(newPrice); ok {
				newPrice = fixedpoint.Value(pin)
			}
		}

		if s.EarnBase {
			newQuantity = fixedpoint.Max(orderQuoteQuantity.Div(newPrice).Sub(baseSellQuantityReduction), s.Market.MinQuantity)
		}
	}

	orderForm := types.SubmitOrder{
		Symbol:      s.Symbol,
		Market:      s.Market,
		Type:        types.OrderTypeLimit,
		Price:       newPrice,
		Side:        newSide,
		TimeInForce: types.TimeInForceGTC,
		Quantity:    newQuantity,
		Tag:         orderTag,
	}

	s.logger.Infof("SUBMIT GRID REVERSE ORDER: %s", orderForm.String())

	if createdOrders, err := s.orderExecutor.SubmitOrders(context.Background(), orderForm); err != nil {
		s.logger.WithError(err).Errorf("can not submit arbitrage order")
	} else {
		s.logger.Infof("order created: %+v", createdOrders)
	}
}

// handleOrderFilled is called when an order status is FILLED
func (s *Strategy) handleOrderFilled(o types.Order) {
	if s.grid == nil {
		return
	}

	s.logger.Infof("GRID ORDER FILLED: %s", o.String())
	s.processFilledOrder(o)
}

func (s *Strategy) checkRequiredInvestmentByQuantity(baseBalance, quoteBalance, quantity, lastPrice fixedpoint.Value, pins []Pin) (requiredBase, requiredQuote fixedpoint.Value, err error) {
	// check more investment budget details
	requiredBase = fixedpoint.Zero
	requiredQuote = fixedpoint.Zero

	// when we need to place a buy-to-sell conversion order, we need to mark the price
	si := -1
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		// TODO: add fee if we don't have the platform token. BNB, OKB or MAX...
		if price.Compare(lastPrice) >= 0 {
			si = i
			// for orders that sell
			// if we still have the base balance
			if requiredBase.Add(quantity).Compare(baseBalance) <= 0 {
				requiredBase = requiredBase.Add(quantity)
			} else if i > 0 { // we do not want to sell at i == 0
				// convert sell to buy quote and add to requiredQuote
				nextLowerPin := pins[i-1]
				nextLowerPrice := fixedpoint.Value(nextLowerPin)
				requiredQuote = requiredQuote.Add(quantity.Mul(nextLowerPrice))
			}
		} else {
			// for orders that buy
			if i+1 == si {
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
	si := -1
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		// TODO: add fee if we don't have the platform token. BNB, OKB or MAX...
		if price.Compare(lastPrice) >= 0 {
			si = i
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
			}
		} else {
			// for orders that buy
			if s.ProfitSpread.IsZero() && i+1 == si {
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

	// quoteInvestment = (p1 * q) + (p2 * q) + (p3 * q) + ....
	// =>
	// quoteInvestment = (p1 + p2 + p3) * q
	// q = quoteInvestment / (p1 + p2 + p3)
	totalQuotePrice := fixedpoint.Zero
	si := -1
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		if price.Compare(lastPrice) >= 0 {
			si = i
			// for orders that sell
			// if we still have the base balance
			// quantity := amount.Div(lastPrice)
			if s.ProfitSpread.Sign() > 0 {
				totalQuotePrice = totalQuotePrice.Add(price)
			} else if i > 0 { // we do not want to sell at i == 0
				// convert sell to buy quote and add to requiredQuote
				nextLowerPin := pins[i-1]
				nextLowerPrice := fixedpoint.Value(nextLowerPin)
				totalQuotePrice = totalQuotePrice.Add(nextLowerPrice)
			}
		} else {
			// for orders that buy
			if s.ProfitSpread.IsZero() && i+1 == si {
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
		sellPrice := price
		if s.ProfitSpread.Sign() > 0 {
			sellPrice = sellPrice.Add(s.ProfitSpread)
		}

		if sellPrice.Compare(lastPrice) < 0 {
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
	s.logger.Infof("grid base investment sell orders: %d", maxNumberOfSellOrders)
	if maxNumberOfSellOrders > 0 {
		s.logger.Infof("grid base investment quantity range: %f <=> %f", minBaseQuantity.Float64(), maxBaseQuantity.Float64())
	}

	// calculate quantity with quote investment
	totalQuotePrice := fixedpoint.Zero
	// quoteInvestment = (p1 * q) + (p2 * q) + (p3 * q) + ....
	// =>
	// quoteInvestment = (p1 + p2 + p3) * q
	// maxBuyQuantity = quoteInvestment / (p1 + p2 + p3)
	si := -1
	for i := len(pins) - 1 - maxNumberOfSellOrders; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		// buy price greater than the last price will trigger taker order.
		if price.Compare(lastPrice) >= 0 {
			si = i

			// when profit spread is set, we count all the grid prices as buy prices
			if s.ProfitSpread.Sign() > 0 {
				totalQuotePrice = totalQuotePrice.Add(price)
			} else if i > 0 {
				// when profit spread is not set
				// we do not want to place sell order at i == 0
				// here we submit an order to convert a buy order into a sell order
				nextLowerPin := pins[i-1]
				nextLowerPrice := fixedpoint.Value(nextLowerPin)
				// requiredQuote = requiredQuote.Add(quantity.Mul(nextLowerPrice))
				totalQuotePrice = totalQuotePrice.Add(nextLowerPrice)
			}
		} else {
			// for orders that buy
			if s.ProfitSpread.IsZero() && i+1 == si {
				continue
			}

			totalQuotePrice = totalQuotePrice.Add(price)
		}
	}

	quoteSideQuantity := quoteInvestment.Div(totalQuotePrice)
	if maxNumberOfSellOrders > 0 {
		return fixedpoint.Min(quoteSideQuantity, maxBaseQuantity), nil
	}

	return quoteSideQuantity, nil
}

func (s *Strategy) newTriggerPriceHandler(ctx context.Context, session *bbgo.ExchangeSession) types.KLineCallback {
	return types.KLineWith(s.Symbol, types.Interval1m, func(k types.KLine) {
		if s.TriggerPrice.Compare(k.High) > 0 || s.TriggerPrice.Compare(k.Low) < 0 {
			return
		}

		if s.grid != nil {
			return
		}

		s.logger.Infof("the last price %f hits triggerPrice %f, opening grid", k.Close.Float64(), s.TriggerPrice.Float64())
		if err := s.openGrid(ctx, session); err != nil {
			s.logger.WithError(err).Errorf("failed to setup grid orders")
			return
		}
	})
}

func (s *Strategy) newStopLossPriceHandler(ctx context.Context, session *bbgo.ExchangeSession) types.KLineCallback {
	return types.KLineWith(s.Symbol, types.Interval1m, func(k types.KLine) {
		if s.StopLossPrice.Compare(k.Low) < 0 {
			return
		}

		s.logger.Infof("last low price %f hits stopLossPrice %f, closing grid", k.Low.Float64(), s.StopLossPrice.Float64())

		if err := s.closeGrid(ctx); err != nil {
			s.logger.WithError(err).Errorf("can not close grid")
			return
		}

		base := s.Position.GetBase()
		if base.Sign() < 0 {
			return
		}

		s.logger.Infof("position base %f > 0, closing position...", base.Float64())
		if err := s.orderExecutor.ClosePosition(ctx, fixedpoint.One, "grid2:stopLoss"); err != nil {
			s.logger.WithError(err).Errorf("can not close position")
			return
		}
	})
}

func (s *Strategy) newTakeProfitHandler(ctx context.Context, session *bbgo.ExchangeSession) types.KLineCallback {
	return types.KLineWith(s.Symbol, types.Interval1m, func(k types.KLine) {
		if s.TakeProfitPrice.Compare(k.High) > 0 {
			return
		}

		s.logger.Infof("last high price %f hits takeProfitPrice %f, closing grid", k.High.Float64(), s.TakeProfitPrice.Float64())

		if err := s.closeGrid(ctx); err != nil {
			s.logger.WithError(err).Errorf("can not close grid")
			return
		}

		base := s.Position.GetBase()
		if base.Sign() < 0 {
			return
		}

		s.logger.Infof("position base %f > 0, closing position...", base.Float64())
		if err := s.orderExecutor.ClosePosition(ctx, fixedpoint.One, "grid2:takeProfit"); err != nil {
			s.logger.WithError(err).Errorf("can not close position")
			return
		}
	})
}

// closeGrid closes the grid orders
func (s *Strategy) closeGrid(ctx context.Context) error {
	bbgo.Sync(ctx, s)

	// now we can cancel the open orders
	s.logger.Infof("canceling grid orders...")

	if err := s.orderExecutor.GracefulCancel(ctx); err != nil {
		return err
	}

	// free the grid object
	s.grid = nil
	return nil
}

func (s *Strategy) newGrid() *Grid {
	grid := NewGrid(s.LowerPrice, s.UpperPrice, fixedpoint.NewFromInt(s.GridNum), s.Market.TickSize)
	grid.CalculateArithmeticPins()
	return grid
}

// openGrid
// 1) if quantity or amount is set, we should use quantity/amount directly instead of using investment amount to calculate.
// 2) if baseInvestment, quoteInvestment is set, then we should calculate the quantity from the given base investment and quote investment.
// TODO: fix sell order placement for profitSpread
func (s *Strategy) openGrid(ctx context.Context, session *bbgo.ExchangeSession) error {
	// grid object guard
	if s.grid != nil {
		return nil
	}

	s.grid = s.newGrid()
	s.logger.Info("OPENING GRID: ", s.grid.String())

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

	submitOrders, err := s.generateGridOrders(totalQuote, totalBase, lastPrice)
	if err != nil {
		return err
	}

	s.debugGridOrders(submitOrders, lastPrice)

	createdOrders, err2 := s.orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err2 != nil {
		return err
	}

	for _, order := range createdOrders {
		s.logger.Info(order.String())
	}

	return nil
}

func (s *Strategy) debugGridOrders(submitOrders []types.SubmitOrder, lastPrice fixedpoint.Value) {
	s.logger.Infof("GRID ORDERS: [")
	for i, order := range submitOrders {
		if i > 0 && lastPrice.Compare(order.Price) >= 0 && lastPrice.Compare(submitOrders[i-1].Price) <= 0 {
			s.logger.Infof("  - LAST PRICE: %f", lastPrice.Float64())
		}

		s.logger.Info("  - ", order.String())
	}
	s.logger.Infof("] END OF GRID ORDERS")
}

func (s *Strategy) generateGridOrders(totalQuote, totalBase, lastPrice fixedpoint.Value) ([]types.SubmitOrder, error) {
	var pins = s.grid.Pins
	var usedBase = fixedpoint.Zero
	var usedQuote = fixedpoint.Zero
	var submitOrders []types.SubmitOrder

	// si is for sell order price index
	var si = len(pins) - 1
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)
		sellPrice := price

		// when profitSpread is set, the sell price is shift upper with the given spread
		if s.ProfitSpread.Sign() > 0 {
			sellPrice = sellPrice.Add(s.ProfitSpread)
		}

		quantity := s.QuantityOrAmount.Quantity
		if quantity.IsZero() {
			quantity = s.QuantityOrAmount.Amount.Div(price)
		}

		// TODO: add fee if we don't have the platform token. BNB, OKB or MAX...
		if price.Compare(lastPrice) >= 0 {
			si = i
			if usedBase.Add(quantity).Compare(totalBase) < 0 {
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Type:        types.OrderTypeLimit,
					Side:        types.SideTypeSell,
					Price:       sellPrice,
					Quantity:    quantity,
					Market:      s.Market,
					TimeInForce: types.TimeInForceGTC,
					Tag:         orderTag,
				})
				usedBase = usedBase.Add(quantity)
			} else if i > 0 {
				// next price
				nextPin := pins[i-1]
				nextPrice := fixedpoint.Value(nextPin)
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Type:        types.OrderTypeLimit,
					Side:        types.SideTypeBuy,
					Price:       nextPrice,
					Quantity:    quantity,
					Market:      s.Market,
					TimeInForce: types.TimeInForceGTC,
					Tag:         orderTag,
				})
				quoteQuantity := quantity.Mul(price)
				usedQuote = usedQuote.Add(quoteQuantity)
			} else if i == 0 {
				// skip i == 0
			}
		} else {
			if s.ProfitSpread.IsZero() && i+1 == si {
				continue
			}

			submitOrders = append(submitOrders, types.SubmitOrder{
				Symbol:      s.Symbol,
				Type:        types.OrderTypeLimit,
				Side:        types.SideTypeBuy,
				Price:       price,
				Quantity:    quantity,
				Market:      s.Market,
				TimeInForce: types.TimeInForceGTC,
				Tag:         orderTag,
			})
			quoteQuantity := quantity.Mul(price)
			usedQuote = usedQuote.Add(quoteQuantity)
		}
	}

	return submitOrders, nil
}

func (s *Strategy) clearOpenOrders(ctx context.Context, session *bbgo.ExchangeSession) error {
	// clear open orders when start
	openOrders, err := session.Exchange.QueryOpenOrders(ctx, s.Symbol)
	if err != nil {
		return err
	}

	err = session.Exchange.CancelOrders(ctx, openOrders...)
	if err != nil {
		return err
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

func calculateMinimalQuoteInvestment(market types.Market, lowerPrice, upperPrice fixedpoint.Value, gridNum int64) fixedpoint.Value {
	num := fixedpoint.NewFromInt(gridNum)
	minimalAmountLowerPrice := fixedpoint.Max(lowerPrice.Mul(market.MinQuantity), market.MinNotional)
	minimalAmountUpperPrice := fixedpoint.Max(upperPrice.Mul(market.MinQuantity), market.MinNotional)
	return fixedpoint.Max(minimalAmountLowerPrice, minimalAmountUpperPrice).Mul(num)
}

func (s *Strategy) checkMinimalQuoteInvestment() error {
	minimalQuoteInvestment := calculateMinimalQuoteInvestment(s.Market, s.LowerPrice, s.UpperPrice, s.GridNum)
	if s.QuoteInvestment.Compare(minimalQuoteInvestment) <= 0 {
		return fmt.Errorf("need at least %f %s for quote investment, %f %s given",
			minimalQuoteInvestment.Float64(),
			s.Market.QuoteCurrency,
			s.QuoteInvestment.Float64(),
			s.Market.QuoteCurrency)
	}
	return nil
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()

	s.session = session

	if service, ok := session.Exchange.(types.ExchangeOrderQueryService); ok {
		s.orderQueryService = service
	}

	s.logger = log.WithFields(logrus.Fields{
		"symbol": s.Symbol,
	})

	s.groupID = util.FNV32(instanceID)
	s.logger.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	if s.ProfitSpread.Sign() > 0 {
		s.ProfitSpread = s.Market.TruncatePrice(s.ProfitSpread)
	}

	if s.GridProfitStats == nil {
		s.GridProfitStats = newGridProfitStats(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ResetPositionWhenStart {
		s.Position.Reset()
	}

	// we need to check the minimal quote investment here, because we need the market info
	if s.QuoteInvestment.Sign() > 0 {
		if err := s.checkMinimalQuoteInvestment(); err != nil {
			return err
		}
	}

	s.historicalTrades = bbgo.NewTradeStore()
	s.historicalTrades.EnablePrune = true
	s.historicalTrades.BindStream(session.UserDataStream)

	orderExecutor := bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	orderExecutor.BindEnvironment(s.Environment)
	orderExecutor.BindProfitStats(s.ProfitStats)
	orderExecutor.Bind()
	orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		s.GridProfitStats.AddTrade(trade)
	})
	orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	orderExecutor.ActiveMakerOrders().OnFilled(s.handleOrderFilled)
	s.orderExecutor = orderExecutor

	// TODO: detect if there are previous grid orders on the order book
	if s.ClearOpenOrdersWhenStart {
		if err := s.clearOpenOrders(ctx, session); err != nil {
			return err
		}
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if s.KeepOrdersWhenShutdown {
			s.logger.Infof("KeepOrdersWhenShutdown is set, will keep the orders on the exchange")
			return
		}

		if err := s.closeGrid(ctx); err != nil {
			s.logger.WithError(err).Errorf("grid graceful order cancel error")
		}
	})

	if !s.TriggerPrice.IsZero() {
		session.MarketDataStream.OnKLineClosed(s.newTriggerPriceHandler(ctx, session))
	}

	if !s.StopLossPrice.IsZero() {
		session.MarketDataStream.OnKLineClosed(s.newStopLossPriceHandler(ctx, session))
	}

	if !s.TakeProfitPrice.IsZero() {
		session.MarketDataStream.OnKLineClosed(s.newTakeProfitHandler(ctx, session))
	}

	session.UserDataStream.OnStart(func() {
		if s.TriggerPrice.IsZero() {
			if err := s.openGrid(ctx, session); err != nil {
				s.logger.WithError(err).Errorf("failed to setup grid orders")
			}
			return
		}
	})

	return nil
}
