package grid2

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "grid2"

const orderTag = "grid2"

var log = logrus.WithField("strategy", ID)

var maxNumberOfOrderTradesQueryTries = 10

const historyRollbackDuration = 3 * 24 * time.Hour
const historyRollbackOrderIdRange = 1000

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type PrettyPins []Pin

func (pp PrettyPins) String() string {
	var ss []string

	for _, p := range pp {
		price := fixedpoint.Value(p)
		ss = append(ss, price.String())
	}

	return fmt.Sprintf("%v", ss)
}

//go:generate mockgen -destination=mocks/order_executor.go -package=mocks . OrderExecutor
type OrderExecutor interface {
	SubmitOrders(ctx context.Context, submitOrders ...types.SubmitOrder) (types.OrderSlice, error)
	ClosePosition(ctx context.Context, percentage fixedpoint.Value, tags ...string) error
	GracefulCancel(ctx context.Context, orders ...types.Order) error
	ActiveMakerOrders() *bbgo.ActiveOrderBook
}

type advancedOrderCancelApi interface {
	CancelAllOrders(ctx context.Context) ([]types.Order, error)
	CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error)
	CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error)
}

//go:generate callbackgen -type Strategy
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

	// BaseGridNum is an optional field used for base investment sell orders
	BaseGridNum int `json:"baseGridNumber,omitempty"`

	AutoRange *types.SimpleDuration `json:"autoRange"`

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

	// RecoverOrdersWhenStart option is used for recovering grid orders
	RecoverOrdersWhenStart bool `json:"recoverOrdersWhenStart"`

	// ClearOpenOrdersWhenStart
	// If this is set, when bbgo started, it will clear the open orders in the same market (by symbol)
	ClearOpenOrdersWhenStart bool `json:"clearOpenOrdersWhenStart"`

	ClearOpenOrdersIfMismatch bool `json:"clearOpenOrdersIfMismatch"`

	ClearDuplicatedPriceOpenOrders bool `json:"clearDuplicatedPriceOpenOrders"`

	// UseCancelAllOrdersApiWhenClose uses a different API to cancel all the orders on the market when closing a grid
	UseCancelAllOrdersApiWhenClose bool `json:"useCancelAllOrdersApiWhenClose"`

	// ResetPositionWhenStart resets the position when the strategy is started
	ResetPositionWhenStart bool `json:"resetPositionWhenStart"`

	// StopIfLessThanMinimalQuoteInvestment stops the strategy if the quote investment does not match
	StopIfLessThanMinimalQuoteInvestment bool `json:"stopIfLessThanMinimalQuoteInvestment"`

	OrderFillDelay types.Duration `json:"orderFillDelay"`

	// PrometheusLabels will be used as the base prometheus labels
	PrometheusLabels prometheus.Labels `json:"prometheusLabels"`

	// OrderGroupID is the group ID used for the strategy instance for canceling orders
	OrderGroupID uint32 `json:"orderGroupID"`

	LogFields logrus.Fields `json:"logFields"`

	// FeeRate is used for calculating the minimal profit spread.
	// it makes sure that your grid configuration is profitable.
	FeeRate fixedpoint.Value `json:"feeRate"`

	SkipSpreadCheck             bool          `json:"skipSpreadCheck"`
	RecoverGridByScanningTrades bool          `json:"recoverGridByScanningTrades"`
	RecoverGridWithin           time.Duration `json:"recoverGridWithin"`

	EnableProfitFixer bool        `json:"enableProfitFixer"`
	FixProfitSince    *types.Time `json:"fixProfitSince"`

	// Debug enables the debug mode
	Debug bool `json:"debug"`

	GridProfitStats *GridProfitStats `persistence:"grid_profit_stats"`
	Position        *types.Position  `persistence:"position"`

	// ExchangeSession is an injection field
	ExchangeSession *bbgo.ExchangeSession

	grid              *Grid
	session           *bbgo.ExchangeSession
	orderQueryService types.ExchangeOrderQueryService

	orderExecutor    OrderExecutor
	historicalTrades *core.TradeStore

	logger *logrus.Entry

	gridReadyCallbacks  []func()
	gridProfitCallbacks []func(stats *GridProfitStats, profit *GridProfit)
	gridClosedCallbacks []func()
	gridErrorCallbacks  []func(err error)

	// filledOrderIDMap is used to prevent processing the same order ID twice.
	filledOrderIDMap *types.SyncOrderMap

	// mu is used for locking the grid object field, avoid double grid opening
	mu sync.Mutex

	tradingCtx, writeCtx context.Context
	cancelWrite          context.CancelFunc

	// this ensures that bbgo.Sync to lock the object
	sync.Mutex
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.AutoRange == nil {
		if s.UpperPrice.IsZero() {
			return errors.New("upperPrice can not be zero, you forgot to set?")
		}

		if s.LowerPrice.IsZero() {
			return errors.New("lowerPrice can not be zero, you forgot to set?")
		}

		if s.UpperPrice.Compare(s.LowerPrice) <= 0 {
			return fmt.Errorf("upperPrice (%s) should not be less than or equal to lowerPrice (%s)", s.UpperPrice.String(), s.LowerPrice.String())
		}
	}

	if s.GridNum == 0 || s.GridNum == 1 {
		return fmt.Errorf("gridNum can not be zero or one")
	}

	if !s.SkipSpreadCheck {
		if err := s.checkSpread(); err != nil {
			return errors.Wrapf(err, "spread is too small, please try to reduce your gridNum or increase the price range (upperPrice and lowerPrice)")
		}
	}

	if !s.QuantityOrAmount.IsSet() && s.QuoteInvestment.IsZero() && s.BaseInvestment.IsZero() {
		return fmt.Errorf("either quantity, amount or quoteInvestment must be set")
	}

	return nil
}

func (s *Strategy) Defaults() error {
	if s.LogFields == nil {
		s.LogFields = logrus.Fields{}
	}

	s.LogFields["symbol"] = s.Symbol
	s.LogFields["strategy"] = ID
	return nil
}

func (s *Strategy) Initialize() error {
	s.filledOrderIDMap = types.NewSyncOrderMap()
	s.logger = log.WithFields(s.LogFields)
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})

	if s.AutoRange != nil {
		interval := s.AutoRange.Interval()
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: interval})
	}
}

// InstanceID returns the instance identifier from the current grid configuration parameters
func (s *Strategy) InstanceID() string {
	id := fmt.Sprintf("%s-%s-size-%d", ID, s.Symbol, s.GridNum)

	if s.AutoRange != nil {
		id += "-autoRange-" + s.AutoRange.String()
	} else {
		id += "-" + s.UpperPrice.String() + "-" + s.LowerPrice.String()
	}

	return id
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
		if err := s.CloseGrid(ctx); err != nil {
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

func (s *Strategy) verifyOrderTrades(o types.Order, trades []types.Trade) bool {
	tq := aggregateTradesQuantity(trades)

	// on MAX: if order.status == filled, it does not mean order.executedQuantity == order.quantity
	// order.executedQuantity can be less than order.quantity
	// so here we use executed quantity to check if the total trade quantity matches to order.executedQuantity
	executedQuantity := o.ExecutedQuantity
	if executedQuantity.IsZero() {
		// fall back to the original quantity if the executed quantity is zero
		executedQuantity = o.Quantity
	}

	// early return here if it matches
	c := tq.Compare(executedQuantity)
	if c == 0 {
		return true
	}

	if c < 0 {
		s.logger.Warnf("order trades missing. expected: %s got: %s",
			executedQuantity.String(),
			tq.String())
		return false
	} else if c > 0 {
		s.logger.Errorf("aggregated trade quantity %s > order executed quantity %s, something is wrong, please check", tq.String(), executedQuantity.String())
		return true
	}

	// shouldn't reach here
	return true
}

// aggregateOrderFee collects the base fee quantity from the given order
// it falls back to query the trades via the RESTful API when the websocket trades are not all received.
func (s *Strategy) aggregateOrderFee(o types.Order) (fixedpoint.Value, string) {
	// try to get the received trades (websocket trades)
	orderTrades := s.historicalTrades.GetOrderTrades(o)
	if len(orderTrades) > 0 {
		s.logger.Infof("GRID: found filled order trades: %+v", orderTrades)
	}

	feeCurrency := s.Market.BaseCurrency
	if o.Side == types.SideTypeSell {
		feeCurrency = s.Market.QuoteCurrency
	}

	for maxTries := maxNumberOfOrderTradesQueryTries; maxTries > 0; maxTries-- {
		// if one of the trades is missing, we need to query the trades from the RESTful API
		if s.verifyOrderTrades(o, orderTrades) {
			// if trades are verified
			fees := collectTradeFee(orderTrades)
			if fee, ok := fees[feeCurrency]; ok {
				return fee, feeCurrency
			}
			return fixedpoint.Zero, feeCurrency
		}

		// if we don't support orderQueryService, then we should just skip
		if s.orderQueryService == nil {
			return fixedpoint.Zero, feeCurrency
		}

		s.logger.Warnf("GRID: missing #%d order trades or missing trade fee, pulling order trades from API", o.OrderID)

		// if orderQueryService is supported, use it to query the trades of the filled order
		apiOrderTrades, err := s.orderQueryService.QueryOrderTrades(context.Background(), types.OrderQuery{
			Symbol:  o.Symbol,
			OrderID: strconv.FormatUint(o.OrderID, 10),
		})
		if err != nil {
			s.logger.WithError(err).Errorf("query #%d order trades error", o.OrderID)
		} else {
			s.logger.Infof("GRID: fetched api #%d order trades: %+v", o.OrderID, apiOrderTrades)
			orderTrades = apiOrderTrades
		}
	}

	// still try to aggregate the trades quantity if we can:
	fees := collectTradeFee(orderTrades)
	if fee, ok := fees[feeCurrency]; ok {
		return fee, feeCurrency
	}

	return fixedpoint.Zero, feeCurrency
}

func (s *Strategy) processFilledOrder(o types.Order) {
	var profit *GridProfit = nil

	// check order fee
	newSide := types.SideTypeSell
	newPrice := o.Price

	executedQuantity := o.ExecutedQuantity
	// A safeguard check, fallback to the original quantity
	if executedQuantity.IsZero() {
		executedQuantity = o.Quantity
	}

	newQuantity := executedQuantity
	executedPrice := o.Price

	if o.ExecutedQuantity.Compare(o.Quantity) != 0 {
		s.logger.Warnf("order #%d is filled, but order executed quantity %s != order quantity %s, something is wrong", o.OrderID, o.ExecutedQuantity, o.Quantity)
	}

	/*
		if o.AveragePrice.Sign() > 0 {
			executedPrice = o.AveragePrice
		}
	*/

	// will be used for calculating quantity
	orderExecutedQuoteAmount := executedQuantity.Mul(executedPrice)
	// round down order executed quote amount to avoid insufficient balance
	orderExecutedQuoteAmount = orderExecutedQuoteAmount.Round(s.Market.PricePrecision, fixedpoint.Down)

	// collect trades for fee
	// fee calculation is used to reduce the order quantity
	// because when 1.0 BTC buy order is filled without FEE token, then we will actually get 1.0 * (1 - feeRate) BTC
	// if we don't reduce the sell quantity, than we might fail to place the sell order
	fee, feeCurrency := s.aggregateOrderFee(o)
	s.logger.Infof("GRID ORDER #%d %s FEE: %s %s",
		o.OrderID, o.Side,
		fee.String(), feeCurrency)

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
			// if it's not using the platform fee currency, reduce the quote quantity for the buy order
			if feeCurrency == s.Market.QuoteCurrency && fee.Sign() > 0 {
				orderExecutedQuoteAmount = orderExecutedQuoteAmount.Sub(fee)
			}

			// for quote amount, always round down with price precision to prevent the quote currency fund locking rounding issue
			origQuoteAmount := orderExecutedQuoteAmount
			orderExecutedQuoteAmount = orderExecutedQuoteAmount.Round(s.Market.PricePrecision, fixedpoint.Down)
			s.logger.Infof("round down %s %s order quote quantity %s to %s by quote precision %d", s.Symbol, newSide, origQuoteAmount.String(), orderExecutedQuoteAmount.String(), s.Market.PricePrecision)

			newQuantity = orderExecutedQuoteAmount.Div(newPrice)

			origQuantity := newQuantity
			newQuantity = newQuantity.Round(s.Market.VolumePrecision, fixedpoint.Down)
			s.logger.Infof("round down %s %s order base quantity %s to %s by base precision %d", s.Symbol, newSide, origQuantity.String(), newQuantity.String(), s.Market.VolumePrecision)

			newQuantity = fixedpoint.Max(newQuantity, s.Market.MinQuantity)
		} else if s.QuantityOrAmount.Quantity.Sign() > 0 {
			newQuantity = s.QuantityOrAmount.Quantity
		}

		// TODO: need to consider sell order fee for the profit calculation
		profit = s.calculateProfit(o, newPrice, newQuantity)

	case types.SideTypeBuy:
		newSide = types.SideTypeSell
		if !s.ProfitSpread.IsZero() {
			newPrice = newPrice.Add(s.ProfitSpread)
		} else {
			if pin, ok := s.grid.NextHigherPin(newPrice); ok {
				newPrice = fixedpoint.Value(pin)
			}
		}

		if feeCurrency == s.Market.BaseCurrency && fee.Sign() > 0 {
			newQuantity = newQuantity.Sub(fee)
		}

		// if EarnBase is enabled, we should sell less to get the same quote amount back
		if s.EarnBase {
			newQuantity = fixedpoint.Max(orderExecutedQuoteAmount.Div(newPrice).Sub(fee), s.Market.MinQuantity)
		}

		// always round down the base quantity for placing sell order to avoid the base currency fund locking issue
		origQuantity := newQuantity
		newQuantity = newQuantity.Round(s.Market.VolumePrecision, fixedpoint.Down)
		s.logger.Infof("round down sell order quantity %s to %s by base quantity precision %d", origQuantity.String(), newQuantity.String(), s.Market.VolumePrecision)
	}

	orderForm := types.SubmitOrder{
		Symbol:        s.Symbol,
		Market:        s.Market,
		Type:          types.OrderTypeLimit,
		Price:         newPrice,
		Side:          newSide,
		TimeInForce:   types.TimeInForceGTC,
		Quantity:      newQuantity,
		Tag:           orderTag,
		GroupID:       s.OrderGroupID,
		ClientOrderID: s.newClientOrderID(),
	}

	s.logger.Infof("SUBMIT GRID REVERSE ORDER: %s", orderForm.String())

	writeCtx := s.getWriteContext()
	createdOrders, err := s.orderExecutor.SubmitOrders(writeCtx, orderForm)
	if err != nil {
		s.logger.WithError(err).Errorf("GRID REVERSE ORDER SUBMISSION ERROR: order: %s", orderForm.String())
		return
	}

	s.logger.Infof("GRID REVERSE ORDER IS CREATED: %+v", createdOrders)

	// we calculate profit only when the order is placed successfully
	if profit != nil {
		s.GridProfitStats.AddProfit(profit)
		s.logger.Infof("GENERATED GRID PROFIT: %+v; TOTAL GRID PROFIT BECOMES: %f", profit, s.GridProfitStats.TotalQuoteProfit.Float64())
		s.EmitGridProfit(s.GridProfitStats, profit)
	}
}

// handleOrderFilled is called when an order status is FILLED
func (s *Strategy) handleOrderFilled(o types.Order) {
	if s.grid == nil {
		s.logger.Warn("grid is not opened yet, skip order update event")
		return
	}

	if s.filledOrderIDMap.Exists(o.OrderID) {
		s.logger.Warnf("duplicated id (%d) of filled order detected", o.OrderID)
		return
	}
	s.filledOrderIDMap.Add(o)

	s.logger.Infof("GRID ORDER FILLED: %s", o.String())
	s.updateFilledOrderMetrics(o)
	s.processFilledOrder(o)
}

func (s *Strategy) checkRequiredInvestmentByQuantity(
	baseBalance, quoteBalance, quantity, lastPrice fixedpoint.Value, pins []Pin,
) (requiredBase, requiredQuote fixedpoint.Value, err error) {
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

func (s *Strategy) checkRequiredInvestmentByAmount(
	baseBalance, quoteBalance, amount, lastPrice fixedpoint.Value, pins []Pin,
) (requiredBase, requiredQuote fixedpoint.Value, err error) {

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

func (s *Strategy) calculateQuoteInvestmentQuantity(
	quoteInvestment, lastPrice fixedpoint.Value, pins []Pin,
) (fixedpoint.Value, error) {
	// quoteInvestment = (p1 * q) + (p2 * q) + (p3 * q) + ....
	// =>
	// quoteInvestment = (p1 + p2 + p3) * q
	// q = quoteInvestment / (p1 + p2 + p3)
	totalQuotePrice := fixedpoint.Zero
	si := len(pins)
	cntOrder := 0
	for i := len(pins) - 1; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)

		if price.Compare(lastPrice) >= 0 {
			si = i

			// do not place sell order on the bottom price
			if i == 0 {
				continue
			}

			// for orders that sell
			// if we still have the base balance
			// quantity := amount.Div(lastPrice)
			if s.ProfitSpread.Sign() > 0 {
				totalQuotePrice = totalQuotePrice.Add(price)
			} else { // we do not want to sell at i == 0
				// convert sell to buy quote and add to requiredQuote
				nextLowerPin := pins[i-1]
				nextLowerPrice := fixedpoint.Value(nextLowerPin)
				totalQuotePrice = totalQuotePrice.Add(nextLowerPrice)
			}

			cntOrder++
		} else {
			// for orders that buy
			if s.ProfitSpread.IsZero() && i+1 == si {
				continue
			}

			// should never place a buy order at the upper price
			if i == len(pins)-1 {
				continue
			}

			totalQuotePrice = totalQuotePrice.Add(price)
			cntOrder++
		}
	}

	orderDusts := fixedpoint.NewFromFloat(math.Pow10(-s.Market.PricePrecision) * float64(cntOrder))
	adjustedQuoteInvestment := quoteInvestment.Sub(orderDusts)
	q := adjustedQuoteInvestment.Div(totalQuotePrice)
	s.logger.Infof("calculateQuoteInvestmentQuantity: adjustedQuoteInvestment=%f sumOfPrice=%f quantity=%f", adjustedQuoteInvestment.Float64(), totalQuotePrice.Float64(), q.Float64())
	return q, nil
}

func (s *Strategy) calculateBaseQuoteInvestmentQuantity(
	quoteInvestment, baseInvestment, lastPrice fixedpoint.Value, pins []Pin,
) (fixedpoint.Value, error) {
	s.logger.Infof("calculating quantity by base/quote investment: %f / %f", baseInvestment.Float64(), quoteInvestment.Float64())
	// q_p1 = q_p2 = q_p3 = q_p4
	// baseInvestment = q_p1 + q_p2 + q_p3 + q_p4 + ....
	// baseInvestment = numberOfSellOrders * q
	// maxBaseQuantity = baseInvestment / numberOfSellOrders
	// if maxBaseQuantity < minQuantity or maxBaseQuantity * priceLowest < minNotional
	// then reduce the numberOfSellOrders
	numberOfSellOrders := s.BaseGridNum

	// if it's not configured, calculate the number of sell orders
	if numberOfSellOrders == 0 {
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

		// avoid placing a sell order above the last price
		if numberOfSellOrders > 0 {
			numberOfSellOrders--
		}
	}

	// if the maxBaseQuantity is less than minQuantity, then we need to reduce the number of the sell orders
	// so that the quantity can be increased.
	baseQuantity := s.Market.TruncateQuantity(
		baseInvestment.Div(
			fixedpoint.NewFromInt(
				int64(numberOfSellOrders))))

	minBaseQuantity := fixedpoint.Max(
		s.Market.MinNotional.Div(s.UpperPrice),
		s.Market.MinQuantity)

	if baseQuantity.Compare(minBaseQuantity) <= 0 {
		baseQuantity = s.Market.RoundUpQuantityByPrecision(minBaseQuantity)
		numberOfSellOrders = int(math.Floor(baseInvestment.Div(baseQuantity).Float64()))
	}

	s.logger.Infof("grid base investment sell orders: %d", numberOfSellOrders)
	s.logger.Infof("grid base investment quantity: %f (base investment) / %d (number of sell orders) = %f (base quantity per order)", baseInvestment.Float64(), numberOfSellOrders, baseQuantity.Float64())

	// calculate quantity with quote investment
	totalQuotePrice := fixedpoint.Zero
	// quoteInvestment = (p1 * q) + (p2 * q) + (p3 * q) + ....
	// =>
	// quoteInvestment = (p1 + p2 + p3) * q
	// maxBuyQuantity = quoteInvestment / (p1 + p2 + p3)
	si := -1
	for i := len(pins) - 1 - numberOfSellOrders; i >= 0; i-- {
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

			// should never place a buy order at the upper price
			if i == len(pins)-1 {
				continue
			}

			totalQuotePrice = totalQuotePrice.Add(price)
		}
	}

	if totalQuotePrice.Sign() > 0 && quoteInvestment.Sign() > 0 {
		quoteSideQuantity := quoteInvestment.Div(totalQuotePrice)
		if numberOfSellOrders > 0 {
			return fixedpoint.Min(quoteSideQuantity, baseQuantity), nil
		}

		return quoteSideQuantity, nil
	}

	return baseQuantity, nil
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

func (s *Strategy) newOrderUpdateHandler(ctx context.Context, session *bbgo.ExchangeSession) func(o types.Order) {
	return func(o types.Order) {
		if s.OrderFillDelay > 0 {
			time.Sleep(s.OrderFillDelay.Duration())
		}

		s.handleOrderFilled(o)

		// sync the profits to redis
		s.debugGridProfitStats("OrderUpdate")
		bbgo.Sync(ctx, s)

		s.updateGridNumOfOrdersMetricsWithLock()
	}
}

func (s *Strategy) newStopLossPriceHandler(ctx context.Context, session *bbgo.ExchangeSession) types.KLineCallback {
	return types.KLineWith(s.Symbol, types.Interval1m, func(k types.KLine) {
		if s.StopLossPrice.Compare(k.Low) < 0 {
			return
		}

		s.logger.Infof("last low price %f hits stopLossPrice %f, closing grid", k.Low.Float64(), s.StopLossPrice.Float64())

		if err := s.CloseGrid(ctx); err != nil {
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

		if err := s.CloseGrid(ctx); err != nil {
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

func (s *Strategy) OpenGrid(ctx context.Context) error {
	return s.openGrid(ctx, s.session)
}

// TODO: make sure the context here is the trading context or the shutdown context?
func (s *Strategy) cancelAll(ctx context.Context) error {
	var werr error

	session := s.session
	if session == nil {
		session = s.ExchangeSession
	}

	service, support := session.Exchange.(advancedOrderCancelApi)
	if s.UseCancelAllOrdersApiWhenClose && !support {
		s.logger.Warnf("advancedOrderCancelApi interface is not implemented, fallback to default graceful cancel, exchange %T", session)
		s.UseCancelAllOrdersApiWhenClose = false
	}

	if s.UseCancelAllOrdersApiWhenClose {
		s.logger.Infof("useCancelAllOrdersApiWhenClose is set, using advanced order cancel api for canceling...")

		for {
			s.logger.Infof("checking %s open orders...", s.Symbol)

			openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, session.Exchange, s.Symbol)
			if err != nil {
				s.logger.WithError(err).Errorf("CancelOrdersByGroupID api call error")
				werr = multierr.Append(werr, err)
			}

			if len(openOrders) == 0 {
				break
			}

			s.logger.Infof("found %d open orders left, using cancel all orders api", len(openOrders))

			s.logger.Infof("using cancal all orders api for canceling grid orders...")
			if err := retry.CancelAllOrdersUntilSuccessful(ctx, service); err != nil {
				s.logger.WithError(err).Errorf("CancelAllOrders api call error")
				werr = multierr.Append(werr, err)
			}

			time.Sleep(1 * time.Second)
		}
	} else {
		if err := s.orderExecutor.GracefulCancel(ctx); err != nil {
			werr = multierr.Append(werr, err)
		}
	}

	return werr
}

// CloseGrid closes the grid orders
func (s *Strategy) CloseGrid(ctx context.Context) error {
	s.logger.Infof("closing %s grid", s.Symbol)

	defer s.EmitGridClosed()

	s.debugGridProfitStats("CloseGrid")
	bbgo.Sync(ctx, s)

	// now we can cancel the open orders
	s.logger.Infof("canceling grid orders...")

	err := s.cancelAll(ctx)

	// free the grid object
	s.setGrid(nil)
	s.updateGridNumOfOrdersMetricsWithLock()
	return err
}

func (s *Strategy) newGrid() *Grid {
	grid := NewGrid(s.LowerPrice, s.UpperPrice, fixedpoint.NewFromInt(s.GridNum), s.Market.TickSize)
	grid.CalculateArithmeticPins()
	return grid
}

// openGrid
// 1) if quantity or amount is set, we should use quantity/amount directly instead of using investment amount to calculate.
// 2) if baseInvestment, quoteInvestment is set, then we should calculate the quantity from the given base investment and quote investment.
func (s *Strategy) openGrid(ctx context.Context, session *bbgo.ExchangeSession) error {
	// grid object guard
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.grid != nil {
		return nil
	}

	grid := s.newGrid()
	s.grid = grid
	s.logger.Info("OPENING GRID: ", s.grid.String())

	lastPrice, err := s.getLastTradePrice(ctx, session)
	if err != nil {
		err2 := errors.Wrap(err, "unable to get the last trade price")
		s.EmitGridError(err2)
		return err2
	}

	// check if base and quote are enough
	var totalBase = fixedpoint.Zero
	var totalQuote = fixedpoint.Zero

	baseBalance, ok := session.Account.Balance(s.Market.BaseCurrency)
	if ok {
		totalBase = baseBalance.Available
	}

	quoteBalance, ok := session.Account.Balance(s.Market.QuoteCurrency)
	if ok {
		totalQuote = quoteBalance.Available
	}

	// shift 1 grid because we will start from the buy order
	// if the buy order is filled, then we will submit another sell order at the higher grid.
	if s.QuantityOrAmount.IsSet() {
		if quantity := s.QuantityOrAmount.Quantity; !quantity.IsZero() {
			if _, _, err2 := s.checkRequiredInvestmentByQuantity(totalBase, totalQuote, lastPrice, s.QuantityOrAmount.Quantity, s.grid.Pins); err != nil {
				s.EmitGridError(err2)
				return err2
			}
		}
		if amount := s.QuantityOrAmount.Amount; !amount.IsZero() {
			if _, _, err2 := s.checkRequiredInvestmentByAmount(totalBase, totalQuote, lastPrice, amount, s.grid.Pins); err != nil {
				s.EmitGridError(err2)
				return err2
			}
		}
	} else {
		// calculate the quantity from the investment configuration
		if !s.BaseInvestment.IsZero() {
			quantity, err2 := s.calculateBaseQuoteInvestmentQuantity(s.QuoteInvestment, s.BaseInvestment, lastPrice, s.grid.Pins)
			if err2 != nil {
				s.EmitGridError(err2)
				return err2
			}

			s.QuantityOrAmount.Quantity = quantity

		} else if !s.QuoteInvestment.IsZero() {
			quantity, err2 := s.calculateQuoteInvestmentQuantity(s.QuoteInvestment, lastPrice, s.grid.Pins)
			if err2 != nil {
				s.EmitGridError(err2)
				return err2
			}

			s.QuantityOrAmount.Quantity = quantity
		}
	}

	// if base investment and quote investment is set, when we should check if the
	// investment configuration is valid with the current balances
	if !s.BaseInvestment.IsZero() && !s.QuoteInvestment.IsZero() {
		if s.BaseInvestment.Compare(totalBase) > 0 {
			err2 := fmt.Errorf("baseInvestment setup %f is greater than the total base balance %f", s.BaseInvestment.Float64(), totalBase.Float64())
			s.EmitGridError(err2)
			return err2
		}
		if s.QuoteInvestment.Compare(totalQuote) > 0 {
			err2 := fmt.Errorf("quoteInvestment setup %f is greater than the total quote balance %f", s.QuoteInvestment.Float64(), totalQuote.Float64())
			s.EmitGridError(err2)
			return err2
		}
	}

	var submitOrders []types.SubmitOrder

	if !s.BaseInvestment.IsZero() || !s.QuoteInvestment.IsZero() {
		submitOrders, err = s.generateGridOrders(s.QuoteInvestment, s.BaseInvestment, lastPrice)
	} else {
		submitOrders, err = s.generateGridOrders(totalQuote, totalBase, lastPrice)
	}

	if err != nil {
		s.EmitGridError(err)
		return err
	}

	s.debugGridOrders(submitOrders, lastPrice)

	writeCtx := s.getWriteContext(ctx)

	createdOrders, err2 := s.orderExecutor.SubmitOrders(writeCtx, submitOrders...)
	if err2 != nil {
		s.EmitGridError(err2)
		return err2
	}

	// try to always emit grid ready
	defer s.EmitGridReady()

	// update the number of orders to metrics
	baseLabels := s.newPrometheusLabels()
	metricsGridNumOfOrders.With(baseLabels).Set(float64(len(createdOrders)))

	var orderIds []uint64

	for _, order := range createdOrders {
		orderIds = append(orderIds, order.OrderID)

		s.logger.Info(order.String())
	}

	sort.Slice(orderIds, func(i, j int) bool {
		return orderIds[i] < orderIds[j]
	})

	if len(orderIds) > 0 {
		s.GridProfitStats.InitialOrderID = orderIds[0]
		s.debugGridProfitStats("openGrid")
		bbgo.Sync(ctx, s)
	}

	s.logger.Infof("ALL GRID ORDERS SUBMITTED")

	s.updateGridNumOfOrdersMetrics(grid)
	return nil
}

func (s *Strategy) updateFilledOrderMetrics(order types.Order) {
	labels := s.newPrometheusLabels()
	labels["side"] = order.Side.String()
	metricsGridFilledOrderPrice.With(labels).Set(order.Price.Float64())
}

func (s *Strategy) updateGridNumOfOrdersMetricsWithLock() {
	if s.mu.TryLock() {
		grid := s.grid
		s.mu.Unlock()
		s.updateGridNumOfOrdersMetrics(grid)
	} else {
		s.logger.Warnf("updateGridNumOfOrdersMetricsWithLock: failed to acquire the lock to update metrics")
	}
}

func (s *Strategy) updateGridNumOfOrdersMetrics(grid *Grid) {
	baseLabels := s.newPrometheusLabels()
	makerOrders := s.orderExecutor.ActiveMakerOrders()
	numOfOrders := makerOrders.NumOfOrders()
	metricsGridNumOfOrders.With(baseLabels).Set(float64(numOfOrders))
	metricsGridLowerPrice.With(baseLabels).Set(s.LowerPrice.Float64())
	metricsGridUpperPrice.With(baseLabels).Set(s.UpperPrice.Float64())
	metricsGridQuoteInvestment.With(baseLabels).Set(s.QuoteInvestment.Float64())
	metricsGridBaseInvestment.With(baseLabels).Set(s.BaseInvestment.Float64())

	if grid != nil {
		gridNum := grid.Size.Int()
		metricsGridNum.With(baseLabels).Set(float64(gridNum))
		numOfMissingOrders := gridNum - 1 - numOfOrders
		metricsGridNumOfMissingOrders.With(baseLabels).Set(float64(numOfMissingOrders))

		var numOfOrdersWithCorrectPrice int
		priceSet := make(map[fixedpoint.Value]struct{})
		for _, order := range makerOrders.Orders() {
			// filter out duplicated prices
			if _, ok := priceSet[order.Price]; ok {
				continue
			}
			priceSet[order.Price] = struct{}{}

			if grid.HasPin(Pin(order.Price)) {
				numOfOrdersWithCorrectPrice++
			}
		}
		numOfMissingOrdersWithCorrectPrice := gridNum - 1 - numOfOrdersWithCorrectPrice
		metricsGridNumOfOrdersWithCorrectPrice.With(baseLabels).Set(float64(numOfOrdersWithCorrectPrice))
		metricsGridNumOfMissingOrdersWithCorrectPrice.With(baseLabels).Set(float64(numOfMissingOrdersWithCorrectPrice))
	}
}

func (s *Strategy) debugGridOrders(submitOrders []types.SubmitOrder, lastPrice fixedpoint.Value) {
	if !s.Debug {
		return
	}

	var sb strings.Builder

	sb.WriteString("GRID ORDERS [\n")
	for i, order := range submitOrders {
		if i > 0 && lastPrice.Compare(order.Price) >= 0 && lastPrice.Compare(submitOrders[i-1].Price) <= 0 {
			sb.WriteString(fmt.Sprintf("  - LAST PRICE: %f\n", lastPrice.Float64()))
		}

		sb.WriteString("  - " + order.String() + "\n")
	}
	sb.WriteString("] END OF GRID ORDERS")

	s.logger.Infof(sb.String())
}

func (s *Strategy) debugOrders(desc string, orders []types.Order) {
	if !s.Debug {
		return
	}

	var sb strings.Builder

	if desc == "" {
		desc = "ORDERS"
	}

	sb.WriteString(desc + " [\n")
	for i, order := range orders {
		sb.WriteString(fmt.Sprintf("  - %d) %s\n", i, order.String()))
	}
	sb.WriteString("]")

	s.logger.Infof(sb.String())
}

func (s *Strategy) debugGridProfitStats(trigger string) {
	if !s.Debug {
		return
	}

	stats := *s.GridProfitStats
	// ProfitEntries may have too many profits, make it nil to readable
	stats.ProfitEntries = nil
	b, err := json.Marshal(stats)
	if err != nil {
		s.logger.WithError(err).Errorf("[%s] failed to debug grid profit stats", trigger)
		return
	}

	s.logger.Infof("trigger %s => grid profit stats : %s", trigger, string(b))
}

func (s *Strategy) debugLog(format string, args ...interface{}) {
	if !s.Debug {
		return
	}

	s.logger.Infof(format, args...)
}
func (s *Strategy) generateGridOrders(totalQuote, totalBase, lastPrice fixedpoint.Value) ([]types.SubmitOrder, error) {
	var pins = s.grid.Pins
	var usedBase = fixedpoint.Zero
	var usedQuote = fixedpoint.Zero
	var submitOrders []types.SubmitOrder

	// si is for sell order price index
	var si = len(pins)
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

		placeSell := price.Compare(lastPrice) >= 0

		// override the relative price position for sell order if BaseGridNum is defined
		if s.BaseGridNum > 0 {
			placeSell = i >= len(pins)-1-s.BaseGridNum
		}

		if placeSell {
			si = i

			// do not place sell order when i == 0 (the bottom of grid)
			if i == 0 {
				continue
			}

			if usedBase.Add(quantity).Compare(totalBase) <= 0 {
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:        s.Symbol,
					Type:          types.OrderTypeLimit,
					Side:          types.SideTypeSell,
					Price:         sellPrice,
					Quantity:      quantity,
					Market:        s.Market,
					TimeInForce:   types.TimeInForceGTC,
					Tag:           orderTag,
					GroupID:       s.OrderGroupID,
					ClientOrderID: s.newClientOrderID(),
				})
				usedBase = usedBase.Add(quantity)
			} else {
				// if we don't have enough base asset
				// then we need to place a buy order at the next price.
				nextPin := pins[i-1]
				nextPrice := fixedpoint.Value(nextPin)
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:        s.Symbol,
					Type:          types.OrderTypeLimit,
					Side:          types.SideTypeBuy,
					Price:         nextPrice,
					Quantity:      quantity,
					Market:        s.Market,
					TimeInForce:   types.TimeInForceGTC,
					Tag:           orderTag,
					GroupID:       s.OrderGroupID,
					ClientOrderID: s.newClientOrderID(),
				})
				quoteQuantity := quantity.Mul(nextPrice)
				usedQuote = usedQuote.Add(quoteQuantity)
			}
		} else {
			// if price spread is not enabled, and we have already placed a sell order index on the top of this price,
			// then we should skip
			if s.ProfitSpread.IsZero() && i+1 == si {
				continue
			}

			// should never place a buy order at the upper price
			if i == len(pins)-1 {
				continue
			}

			quoteQuantity := quantity.Mul(price)

			if usedQuote.Add(quoteQuantity).Compare(totalQuote) > 0 {
				s.logger.Warnf("used quote %f > total quote %f, this should not happen", usedQuote.Add(quoteQuantity).Float64(), totalQuote.Float64())
				continue
			}

			submitOrders = append(submitOrders, types.SubmitOrder{
				Symbol:        s.Symbol,
				Type:          types.OrderTypeLimit,
				Side:          types.SideTypeBuy,
				Price:         price,
				Quantity:      quantity,
				Market:        s.Market,
				TimeInForce:   types.TimeInForceGTC,
				Tag:           orderTag,
				GroupID:       s.OrderGroupID,
				ClientOrderID: s.newClientOrderID(),
			})
			usedQuote = usedQuote.Add(quoteQuantity)
		}
	}

	return submitOrders, nil
}

func (s *Strategy) clearOpenOrders(ctx context.Context, session *bbgo.ExchangeSession) error {
	// clear open orders when start
	openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, session.Exchange, s.Symbol)
	if err != nil {
		return err
	}

	return retry.CancelOrdersUntilSuccessful(ctx, session.Exchange, openOrders...)
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

func calculateMinimalQuoteInvestment(market types.Market, grid *Grid) fixedpoint.Value {
	// upperPrice for buy order
	lowerPrice := grid.LowerPrice
	minQuantity := fixedpoint.Max(market.MinNotional.Div(lowerPrice), market.MinQuantity)

	var pins = grid.Pins
	var totalQuote = fixedpoint.Zero
	for i := len(pins) - 2; i >= 0; i-- {
		pin := pins[i]
		price := fixedpoint.Value(pin)
		totalQuote = totalQuote.Add(price.Mul(minQuantity))
	}

	return totalQuote
}

func (s *Strategy) checkMinimalQuoteInvestment(grid *Grid) error {
	minimalQuoteInvestment := calculateMinimalQuoteInvestment(s.Market, grid)
	if s.QuoteInvestment.Compare(minimalQuoteInvestment) <= 0 {
		return fmt.Errorf("need at least %f %s for quote investment, %f %s given",
			minimalQuoteInvestment.Float64(),
			s.Market.QuoteCurrency,
			s.QuoteInvestment.Float64(),
			s.Market.QuoteCurrency)
	}
	return nil
}

func (s *Strategy) recoverGridWithOpenOrders(
	ctx context.Context, historyService types.ExchangeTradeHistoryService, openOrders []types.Order,
) error {
	grid := s.newGrid()

	s.logger.Infof("GRID RECOVER: %s", grid.String())

	lastOrderID := uint64(1)
	now := time.Now()
	firstOrderTime := now.AddDate(0, 0, -7)
	lastOrderTime := firstOrderTime
	if since, until, ok := scanOrderCreationTimeRange(openOrders); ok {
		firstOrderTime = since
		lastOrderTime = until
	}
	_ = lastOrderTime

	// for MAX exchange we need the order ID to query the closed order history
	if s.GridProfitStats != nil && s.GridProfitStats.InitialOrderID > 0 {
		lastOrderID = s.GridProfitStats.InitialOrderID
		s.logger.Infof("found initial order id #%d from grid stats", lastOrderID)
	} else {
		if oid, ok := findEarliestOrderID(openOrders); ok {
			lastOrderID = oid
			s.logger.Infof("found earliest order id #%d from open orders", lastOrderID)
		}
	}

	// Allocate a local order book for querying the history orders
	orderBook := bbgo.NewActiveOrderBook(s.Symbol)

	// Ensure that orders are grid orders
	// The price must be at the grid pin
	gridOrders := grid.FilterOrders(openOrders)
	for _, gridOrder := range gridOrders {
		orderBook.Add(gridOrder)
	}

	// if all open orders are the grid orders, then we don't have to recover
	s.logger.Infof("GRID RECOVER: verifying pins %v", PrettyPins(grid.Pins))
	missingPrices := scanMissingPinPrices(orderBook, grid.Pins)
	if numMissing := len(missingPrices); numMissing <= 1 {
		s.logger.Infof("GRID RECOVER: no missing grid prices, stop re-playing order history")
		s.addOrdersToActiveOrderBook(gridOrders)
		s.setGrid(grid)
		s.EmitGridReady()
		s.updateGridNumOfOrdersMetricsWithLock()
		return nil
	} else {
		s.logger.Infof("GRID RECOVER: found missing prices: %v", missingPrices)
		// Note that for MAX Exchange, the order history API only uses fromID parameter to query history order.
		// The time range does not matter.
		// TODO: handle context correctly
		startTime := firstOrderTime
		endTime := now
		maxTries := 5
		localHistoryRollbackDuration := historyRollbackDuration
		for maxTries > 0 {
			maxTries--
			if err := s.replayOrderHistory(ctx, grid, orderBook, historyService, startTime, endTime, lastOrderID); err != nil {
				return err
			}

			// Verify if there are still missing prices
			missingPrices = scanMissingPinPrices(orderBook, grid.Pins)
			if len(missingPrices) <= 1 {
				// skip this order history loop and start recovering
				break
			}

			// history rollback range
			startTime = startTime.Add(-localHistoryRollbackDuration)
			if newFromOrderID := lastOrderID - historyRollbackOrderIdRange; newFromOrderID > 1 {
				lastOrderID = newFromOrderID
			}

			s.logger.Infof("GRID RECOVER: there are still more than two missing orders, rolling back query start time to earlier time point %s, fromID %d", startTime.String(), lastOrderID)
			localHistoryRollbackDuration = localHistoryRollbackDuration * 2
		}
	}

	debugGrid(s.logger, grid, orderBook)

	// note that the tmpOrders contains FILLED and NEW orders
	tmpOrders := orderBook.Orders()

	// if all orders on the order book are active orders, we don't need to recover.
	if isCompleteGridOrderBook(orderBook, s.GridNum) {
		s.logger.Infof("GRID RECOVER: all orders are active orders, do not need recover")
		s.addOrdersToActiveOrderBook(gridOrders)
		s.setGrid(grid)
		s.EmitGridReady()
		s.updateGridNumOfOrdersMetricsWithLock()
		return nil
	}

	// for reverse order recovering, we need the orders to be sort by update time ascending-ly
	types.SortOrdersUpdateTimeAscending(tmpOrders)

	// we will only submit reverse orders for filled orders
	filledOrders := types.OrdersFilled(tmpOrders)

	// if the number of FILLED orders and NEW orders equals to GridNum, then we need to remove an extra filled order for the replay events
	if len(tmpOrders) == int(s.GridNum) && len(filledOrders) > 0 {
		// remove the latest updated order because it's near the empty slot
		filledOrders = filledOrders[1:]
	}

	s.logger.Infof("GRID RECOVER: found %d/%d filled grid orders, gridNumber=%d, will re-replay the order event in the following order:", len(filledOrders), len(tmpOrders), int(s.GridNum))
	for i, o := range filledOrders {
		s.logger.Infof("%d) %s", i+1, o.String())
	}

	// before we re-play the orders,
	// we need to add these open orders to the active order book
	s.addOrdersToActiveOrderBook(gridOrders)
	s.setGrid(grid)
	s.EmitGridReady()
	s.updateGridNumOfOrdersMetricsWithLock()

	for i := range filledOrders {
		// avoid using the iterator
		o := filledOrders[i]
		s.processFilledOrder(o)
		time.Sleep(100 * time.Millisecond)
	}

	// wait for the reverse order to be placed
	time.Sleep(2 * time.Second)

	s.logger.Infof("GRID RECOVER COMPLETE")

	debugGrid(s.logger, grid, s.orderExecutor.ActiveMakerOrders())

	s.updateGridNumOfOrdersMetricsWithLock()
	return nil
}

func (s *Strategy) addOrdersToActiveOrderBook(gridOrders []types.Order) {
	activeOrderBook := s.orderExecutor.ActiveMakerOrders()
	for _, gridOrder := range gridOrders {
		// put the order back to the active order book so that we can receive order update
		activeOrderBook.Add(gridOrder)
	}
}

func (s *Strategy) setGrid(grid *Grid) {
	s.mu.Lock()
	s.grid = grid
	s.mu.Unlock()
}

func (s *Strategy) getGrid() *Grid {
	s.mu.Lock()
	grid := s.grid
	s.mu.Unlock()
	return grid
}

// replayOrderHistory queries the closed order history from the API and rebuild the orderbook from the order history.
// startTime, endTime is the time range of the order history.
func (s *Strategy) replayOrderHistory(
	ctx context.Context, grid *Grid, orderBook *bbgo.ActiveOrderBook, historyService types.ExchangeTradeHistoryService,
	startTime, endTime time.Time, lastOrderID uint64,
) error {
	// a simple guard, in reality, this startTime is not possible to exceed the endTime
	// because the queries closed orders might still in the range.
	orderIdChanged := true
	for startTime.Before(endTime) && orderIdChanged {
		closedOrders, err := historyService.QueryClosedOrders(ctx, s.Symbol, startTime, endTime, lastOrderID)
		if err != nil {
			return err
		}

		// need to prevent infinite loop for:
		// if there is only one order and the order creation time matches our startTime
		if len(closedOrders) == 0 || len(closedOrders) == 1 && closedOrders[0].OrderID == lastOrderID {
			break
		}

		// for each closed order, if it's newer than the open order's update time, we will update it.
		orderIdChanged = false
		for _, closedOrder := range closedOrders {
			if closedOrder.OrderID > lastOrderID {
				lastOrderID = closedOrder.OrderID
				orderIdChanged = true
			}

			// skip orders that are not limit order
			if closedOrder.Type != types.OrderTypeLimit {
				continue
			}

			// skip canceled orders (?)
			if closedOrder.Status == types.OrderStatusCanceled {
				continue
			}

			creationTime := closedOrder.CreationTime.Time()
			if creationTime.After(startTime) {
				startTime = creationTime
			}

			// skip non-grid order prices

			if !grid.HasPrice(closedOrder.Price) {
				continue
			}

			existingOrder := orderBook.Lookup(func(o types.Order) bool {
				return o.Price.Eq(closedOrder.Price)
			})

			if existingOrder == nil {
				orderBook.Add(closedOrder)
			} else {
				// To update order, we need to remove the old order, because it's using order ID as the key of the map.
				if creationTime.After(existingOrder.CreationTime.Time()) {
					orderBook.Remove(*existingOrder)
					orderBook.Add(closedOrder)
				}
			}
		}
	}

	return nil
}

// isCompleteGridOrderBook checks if the number of open orders == gridNum - 1 and all orders are active order
func isCompleteGridOrderBook(orderBook *bbgo.ActiveOrderBook, gridNum int64) bool {
	tmpOrders := orderBook.Orders()
	activeOrders := types.OrdersActive(tmpOrders)
	return len(activeOrders) == int(gridNum)-1
}

func findEarliestOrderID(orders []types.Order) (uint64, bool) {
	if len(orders) == 0 {
		return 0, false
	}

	earliestOrderID := orders[0].OrderID
	for _, o := range orders {
		if o.OrderID < earliestOrderID {
			earliestOrderID = o.OrderID
		}
	}

	return earliestOrderID, true
}

// scanOrderCreationTimeRange finds the earliest creation time and the latest creation time from the given orders
func scanOrderCreationTimeRange(orders []types.Order) (time.Time, time.Time, bool) {
	if len(orders) == 0 {
		return time.Time{}, time.Time{}, false
	}

	firstOrderTime := orders[0].CreationTime.Time()
	lastOrderTime := firstOrderTime
	for _, o := range orders {
		createTime := o.CreationTime.Time()
		if createTime.Before(firstOrderTime) {
			firstOrderTime = createTime
		} else if createTime.After(lastOrderTime) {
			lastOrderTime = createTime
		}
	}

	return firstOrderTime, lastOrderTime, true
}

// scanMissingPinPrices finds the missing grid order prices
func scanMissingPinPrices(orderBook *bbgo.ActiveOrderBook, pins []Pin) PriceMap {
	// Add all open orders to the local order book
	gridPrices := make(PriceMap)
	missingPrices := make(PriceMap)
	for _, pin := range pins {
		price := fixedpoint.Value(pin)
		gridPrices[price.String()] = price
		existingOrder := orderBook.Lookup(func(o types.Order) bool {
			return o.Price.Compare(price) == 0
		})
		if existingOrder == nil {
			missingPrices[price.String()] = price
		}
	}

	return missingPrices
}

func (s *Strategy) newPrometheusLabels() prometheus.Labels {
	labels := prometheus.Labels{
		"exchange": "default",
		"symbol":   s.Symbol,
	}

	if s.session != nil {
		labels["exchange"] = s.session.Name
	}

	if s.PrometheusLabels == nil {
		return labels
	}

	return mergeLabels(s.PrometheusLabels, labels)
}

func (s *Strategy) CleanUp(ctx context.Context) error {
	if s.ExchangeSession != nil {
		s.session = s.ExchangeSession
	}

	_ = s.Initialize()

	defer s.EmitGridClosed()
	return s.cancelAll(ctx)
}

func (s *Strategy) getWriteContext(fallbackCtxList ...context.Context) context.Context {
	if s.writeCtx != nil {
		return s.writeCtx
	}

	for _, c := range fallbackCtxList {
		if c != nil {
			return c
		}
	}

	if s.tradingCtx != nil {
		return s.tradingCtx
	}

	// final fallback to context background
	return context.Background()
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()

	// allocate a context for write operation (submitting orders)
	s.tradingCtx = ctx
	s.writeCtx, s.cancelWrite = context.WithCancel(ctx)

	s.session = session

	if service, ok := session.Exchange.(types.ExchangeOrderQueryService); ok {
		s.orderQueryService = service
	}

	if s.OrderGroupID == 0 {
		s.OrderGroupID = util.FNV32(instanceID) % math.MaxInt32
	}

	if s.AutoRange != nil {
		indicatorSet := session.StandardIndicatorSet(s.Symbol)
		interval := s.AutoRange.Interval()
		pivotLow := indicatorSet.PivotLow(types.IntervalWindow{Interval: interval, Window: s.AutoRange.Num})
		pivotHigh := indicatorSet.PivotHigh(types.IntervalWindow{Interval: interval, Window: s.AutoRange.Num})
		s.UpperPrice = fixedpoint.NewFromFloat(pivotHigh.Last(0))
		s.LowerPrice = fixedpoint.NewFromFloat(pivotLow.Last(0))
		s.logger.Infof("autoRange is enabled, using pivot high %f and pivot low %f", s.UpperPrice.Float64(), s.LowerPrice.Float64())
	}

	if s.ProfitSpread.Sign() > 0 {
		s.ProfitSpread = s.Market.TruncatePrice(s.ProfitSpread)
	}

	if s.GridProfitStats == nil {
		s.GridProfitStats = newGridProfitStats(s.Market)
	}

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	// initialize and register prometheus metrics
	if s.PrometheusLabels != nil {
		initMetrics(labelKeys(s.PrometheusLabels))
	} else {
		initMetrics(nil)
	}
	registerMetrics()

	if s.ResetPositionWhenStart {
		s.Position.Reset()
	}

	// we need to check the minimal quote investment here, because we need the market info
	if s.QuoteInvestment.Sign() > 0 {
		grid := s.newGrid()
		if err := s.checkMinimalQuoteInvestment(grid); err != nil {
			if s.StopIfLessThanMinimalQuoteInvestment {
				s.logger.WithError(err).Errorf("check minimal quote investment failed, market info: %+v", s.Market)
				return err
			} else {
				// if no, just warning
				s.logger.WithError(err).Warnf("minimal quote investment may be not enough, market info: %+v", s.Market)
			}
		}
	}

	s.historicalTrades = core.NewTradeStore()
	s.historicalTrades.EnablePrune = true
	s.historicalTrades.BindStream(session.UserDataStream)

	orderExecutor := bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	orderExecutor.BindEnvironment(s.Environment)
	orderExecutor.Bind()
	orderExecutor.TradeCollector().OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		s.GridProfitStats.AddTrade(trade)
	})
	orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		s.debugGridProfitStats("OnPositionUpdate")
		bbgo.Sync(ctx, s)
	})
	orderExecutor.ActiveMakerOrders().OnFilled(s.newOrderUpdateHandler(ctx, session))

	if s.logger != nil {
		orderExecutor.SetLogger(s.logger)
	}

	s.orderExecutor = orderExecutor

	s.OnGridProfit(func(stats *GridProfitStats, profit *GridProfit) {
		if profit != nil {
			bbgo.Notify(profit)
		}
		bbgo.Notify(stats)
	})

	s.OnGridProfit(func(stats *GridProfitStats, profit *GridProfit) {
		labels := s.newPrometheusLabels()
		metricsGridProfit.With(labels).Set(stats.TotalQuoteProfit.Float64())
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if s.cancelWrite != nil {
			s.cancelWrite()
		}

		if s.KeepOrdersWhenShutdown {
			s.logger.Infof("keepOrdersWhenShutdown is set, will keep the orders on the exchange")
			return
		}

		if err := s.CloseGrid(ctx); err != nil {
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

	// detect if there are previous grid orders on the order book
	session.UserDataStream.OnStart(func() {
		if s.ClearOpenOrdersWhenStart {
			s.logger.Infof("clearOpenOrdersWhenStart is set, clearing open orders...")
			if err := s.clearOpenOrders(ctx, session); err != nil {
				s.logger.WithError(err).Errorf("clearOpenOrdersWhenStart error")
			}
		}

		if s.ClearOpenOrdersIfMismatch {
			s.logger.Infof("clearOpenOrdersIfMismatch is set, checking mismatched orders...")
			mismatch, err := s.openOrdersMismatches(ctx, session)
			if err != nil {
				s.logger.WithError(err).Errorf("clearOpenOrdersIfMismatch error")
			} else if mismatch {
				if err2 := s.clearOpenOrders(ctx, session); err2 != nil {
					s.logger.WithError(err2).Errorf("clearOpenOrders error")
				}
			}
		}

		if s.ClearDuplicatedPriceOpenOrders {
			s.logger.Infof("clearDuplicatedPriceOpenOrders is set, finding duplicated open orders...")
			if err := s.cancelDuplicatedPriceOpenOrders(ctx, session); err != nil {
				s.logger.WithError(err).Errorf("cancelDuplicatedPriceOpenOrders error")
			}
		}
	})

	// if TriggerPrice is zero, that means we need to open the grid when start up
	if s.TriggerPrice.IsZero() {
		// must call the openGrid method inside the OnStart callback because
		// it needs to receive the trades from the user data stream
		//
		// should try to avoid blocking the user data stream
		// callbacks are blocking operation
		session.UserDataStream.OnStart(func() {
			s.logger.Infof("user data stream started, initializing grid...")

			if !bbgo.IsBackTesting {
				go time.AfterFunc(3*time.Second, func() {
					s.startProcess(ctx, session)
				})
			} else {
				s.startProcess(ctx, session)
			}
		})
	}

	session.UserDataStream.OnConnect(func() {
		if !bbgo.IsBackTesting {
			// callback may block the stream execution, so we spawn the recover function to the background
			// add (5 seconds + random <10 seconds jitter) delay
			go time.AfterFunc(util.MillisecondsJitter(5*time.Second, 1000*10), func() {
				s.recoverActiveOrders(ctx, session)
			})
		}
	})

	return nil
}

func (s *Strategy) startProcess(ctx context.Context, session *bbgo.ExchangeSession) {
	s.debugGridProfitStats("startProcess")
	if s.RecoverOrdersWhenStart {
		// do recover only when triggerPrice is not set and not in the back-test mode
		s.logger.Infof("recoverWhenStart is set, trying to recover grid orders...")
		if err := s.recoverGrid(ctx, session); err != nil {
			// if recover fail, return and do not open grid
			s.logger.WithError(err).Error("failed to start process, recover error")
			s.EmitGridError(errors.Wrapf(err, "failed to start process, recover error"))
			return
		}
	}

	// avoid using goroutine here for back-test
	if err := s.openGrid(ctx, session); err != nil {
		s.EmitGridError(errors.Wrapf(err, "failed to start process, setup grid orders error"))
		return
	}
}

func (s *Strategy) recoverGrid(ctx context.Context, session *bbgo.ExchangeSession) error {
	if s.RecoverGridByScanningTrades {
		s.debugLog("recovering grid by scanning trades")
		return s.recoverByScanningTrades(ctx, session)
	}

	s.debugLog("recovering grid by scanning orders")
	return s.recoverByScanningOrders(ctx, session)
}

func (s *Strategy) recoverByScanningOrders(ctx context.Context, session *bbgo.ExchangeSession) error {
	openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, session.Exchange, s.Symbol)
	if err != nil {
		return err
	}

	// do recover only when openOrders > 0
	if len(openOrders) == 0 {
		s.logger.Warn("0 open orders, skip recovery process")
		return nil
	}

	s.logger.Infof("found %d open orders left on the %s order book", len(openOrders), s.Symbol)

	historyService, implemented := session.Exchange.(types.ExchangeTradeHistoryService)
	if !implemented {
		s.logger.Warn("ExchangeTradeHistoryService is not implemented, can not recover grid")
		return nil
	}

	if err := s.recoverGridWithOpenOrders(ctx, historyService, openOrders); err != nil {
		return errors.Wrap(err, "grid recover error")
	}

	return nil
}

// openOrdersMismatches verifies if the open orders are on the grid pins
// return true if mismatches
func (s *Strategy) openOrdersMismatches(ctx context.Context, session *bbgo.ExchangeSession) (bool, error) {
	openOrders, err := session.Exchange.QueryOpenOrders(ctx, s.Symbol)
	if err != nil {
		return false, err
	}

	if len(openOrders) == 0 {
		return false, nil
	}

	grid := s.newGrid()
	for _, o := range openOrders {
		// if any of the open order is not on the grid, or out of the range
		// we should cancel all of them
		if !grid.HasPrice(o.Price) || grid.OutOfRange(o.Price) {
			return true, nil
		}
	}

	return false, nil
}

func (s *Strategy) cancelDuplicatedPriceOpenOrders(ctx context.Context, session *bbgo.ExchangeSession) error {
	openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, session.Exchange, s.Symbol)
	if err != nil {
		return err
	}

	if len(openOrders) == 0 {
		return nil
	}

	dupOrders := s.findDuplicatedPriceOpenOrders(openOrders)

	if len(dupOrders) > 0 {
		s.debugOrders("DUPLICATED ORDERS", dupOrders)
		return session.Exchange.CancelOrders(ctx, dupOrders...)
	}

	s.logger.Infof("no duplicated order found")
	return nil
}

func (s *Strategy) findDuplicatedPriceOpenOrders(openOrders []types.Order) (dupOrders []types.Order) {
	orderBook := bbgo.NewActiveOrderBook(s.Symbol)
	for _, openOrder := range openOrders {
		existingOrder := orderBook.Lookup(func(o types.Order) bool {
			return o.Price.Compare(openOrder.Price) == 0
		})

		if existingOrder != nil {
			// found duplicated order
			// compare creation time and remove the latest created order
			// if the creation time equals, then we can just cancel one of them
			s.debugOrders(
				fmt.Sprintf("found duplicated order at price %s, comparing orders", openOrder.Price.String()),
				[]types.Order{*existingOrder, openOrder})

			dupOrder := *existingOrder
			if openOrder.CreationTime.After(existingOrder.CreationTime.Time()) {
				dupOrder = openOrder
			} else if openOrder.CreationTime.Before(existingOrder.CreationTime.Time()) {
				// override the existing order and take the existing order as a duplicated one
				orderBook.Add(openOrder)
			}

			dupOrders = append(dupOrders, dupOrder)
		} else {
			orderBook.Add(openOrder)
		}
	}

	return dupOrders
}

func (s *Strategy) newClientOrderID() string {
	if s.session != nil && s.session.ExchangeName == types.ExchangeMax {
		return uuid.New().String()
	}
	return ""
}

func (s *Strategy) recoverActiveOrders(ctx context.Context, session *bbgo.ExchangeSession) {
	s.logger.Infof("recovering active orders after websocket connect")

	grid := s.getGrid()
	if grid == nil {
		return
	}

	// this lock avoids recovering the active orders while the openGrid is executing
	s.mu.Lock()
	defer s.mu.Unlock()

	// TODO: move this logics into the active maker orders component, like activeOrders.Sync(ctx)
	activeOrderBook := s.orderExecutor.ActiveMakerOrders()
	activeOrders := activeOrderBook.Orders()
	if len(activeOrders) == 0 {
		return
	}

	s.logger.Infof("found %d active orders to update...", len(activeOrders))
	for _, o := range activeOrders {
		s.logger.Infof("updating %d order...", o.OrderID)

		var updatedOrder *types.Order
		err := retry.GeneralBackoff(ctx, func() error {
			var err error
			updatedOrder, err = s.orderQueryService.QueryOrder(ctx, types.OrderQuery{
				Symbol:  o.Symbol,
				OrderID: strconv.FormatUint(o.OrderID, 10),
			})
			return err
		})

		if err != nil {
			s.logger.WithError(err).Errorf("unable to query order")
			return
		}

		activeOrderBook.Update(*updatedOrder)
	}
}
