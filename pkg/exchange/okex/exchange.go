package okex

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/envvar"
	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var (
	// clientOrderIdRegex combine of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderIdRegex = regexp.MustCompile("^[a-zA-Z0-9]{0,32}$")

	// Rate Limit: 20 requests per 2 seconds, Rate limit rule: IP + instrumentType.
	// Currently, calls are not made very frequently, so only IP is considered.
	queryMarketLimiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
	// Rate Limit: 20 requests per 2 seconds, Rate limit rule: IP
	queryTickerLimiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
	// Rate Limit: 20 requests per 2 seconds, Rate limit rule: IP
	queryTickersLimiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
	// Rate Limit: 10 requests per 2 seconds, Rate limit rule: UserID
	queryAccountLimiter = rate.NewLimiter(rate.Every(200*time.Millisecond), 1)
	// Rate Limit: 60 requests per 2 seconds, Rate limit rule (except Options): UserID + Instrument ID.
	// TODO: support UserID + Instrument ID
	placeOrderLimiter = rate.NewLimiter(rate.Every(33*time.Millisecond), 1)
	// Rate Limit: 60 requests per 2 seconds, Rate limit rule (except Options): UserID + Instrument ID
	// TODO: support UserID + Instrument ID
	batchCancelOrderLimiter = rate.NewLimiter(rate.Every(33*time.Millisecond), 1)
	// Rate Limit: 60 requests per 2 seconds, Rate limit rule: UserID
	queryOpenOrderLimiter = rate.NewLimiter(rate.Every(33*time.Millisecond), 1)
	// Rate Limit: 20 requests per 2 seconds, Rate limit rule: UserID
	queryClosedOrderRateLimiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
	// Rate Limit: 40 requests per 2 seconds, Rate limit rule: IP
	queryKLineLimiter = rate.NewLimiter(rate.Every(50*time.Millisecond), 1)
)

const (
	ID = "okex"

	// PlatformToken is the platform currency of OKEx, pre-allocate static string here
	PlatformToken = "OKB"

	defaultQueryLimit = 100

	maxHistoricalDataQueryPeriod = 90 * 24 * time.Hour
	threeDaysHistoricalPeriod    = 3 * 24 * time.Hour
)

// Enable dual side position mode (hedge mode) for futures trading
var dualSidePosition = false

var log = logrus.WithFields(logrus.Fields{
	"exchange": ID,
})

var ErrSymbolRequired = errors.New("symbol is a required parameter")

func init() {
	if val, ok := envvar.Bool("OKEX_ENABLE_FUTURES_HEDGE_MODE"); ok {
		dualSidePosition = val
	}
}

type Exchange struct {
	types.MarginSettings
	types.FuturesSettings

	key, secret, passphrase, brokerId string

	client      *okexapi.RestClient
	timeNowFunc func() time.Time
}

type Option func(exchange *Exchange)

func WithBrokerId(id string) Option {
	return func(exchange *Exchange) {
		exchange.brokerId = id
	}
}

func New(key, secret, passphrase string, opts ...Option) *Exchange {
	client := okexapi.NewClient()

	log.Debugf("creating new okex rest client with base url: %s", okexapi.RestBaseURL)

	if len(key) > 0 && len(secret) > 0 {
		client.Auth(key, secret, passphrase)
	}

	ex := &Exchange{
		key:         key,
		secret:      secret,
		passphrase:  passphrase,
		client:      client,
		timeNowFunc: time.Now,
		brokerId:    defaultBrokerId,
	}

	if str, ok := os.LookupEnv("OKEX_BROKER_ID"); ok {
		ex.brokerId = str
	}

	for _, o := range opts {
		o(ex)
	}

	return ex
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeOKEx
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	if err := queryMarketLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("markets rate limiter wait error: %w", err)
	}

	req := e.client.NewGetInstrumentsInfoRequest()
	if e.IsFutures {
		req.InstType(okexapi.InstrumentTypeSwap)
	}

	instruments, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, instrument := range instruments {
		symbol := toGlobalSymbol(instrument.InstrumentID)
		baseCurrency, quoteCurrency := toGlobalCurrency(instrument)
		market := types.Market{
			Exchange:    types.ExchangeOKEx,
			Symbol:      symbol,
			LocalSymbol: instrument.InstrumentID,

			QuoteCurrency: quoteCurrency,
			BaseCurrency:  baseCurrency,

			// convert tick size OKEx to precision
			PricePrecision:  instrument.TickSize.NumFractionalDigits(),
			VolumePrecision: instrument.LotSize.NumFractionalDigits(),

			// TickSize: OKEx's price tick, for BTC-USDT it's "0.1"
			TickSize: instrument.TickSize,

			// Quantity step size, for BTC-USDT, it's "0.00000001"
			StepSize: instrument.LotSize,

			// for BTC-USDT, it's "0.00001"
			MinQuantity: instrument.MinSize,

			// OKEx does not offer minimal notional, use 1 USD here.
			MinNotional: fixedpoint.One,
			MinAmount:   fixedpoint.One,

			ContractValue: instrument.ContractValue,
		}
		markets[symbol] = market
	}

	e.syncLocalSymbolMap(markets)

	return markets, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	if err := queryTickerLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("ticker rate limiter wait error: %w", err)
	}

	symbol = e.getInstrumentId(symbol)
	marketTicker, err := e.client.NewGetTickerRequest().InstId(symbol).Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(marketTicker) != 1 {
		return nil, fmt.Errorf("unexpected length of %s market ticker, got: %v", symbol, marketTicker)
	}

	return toGlobalTicker(marketTicker[0]), nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbols ...string) (map[string]types.Ticker, error) {
	if err := queryTickersLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("tickers rate limiter wait error: %w", err)
	}

	req := e.client.NewGetTickersRequest()
	if e.IsFutures {
		req.InstType(okexapi.InstrumentTypeSwap)
	}

	marketTickers, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	tickers := make(map[string]types.Ticker)
	for _, marketTicker := range marketTickers {
		symbol := toGlobalSymbol(marketTicker.InstrumentID)
		ticker := toGlobalTicker(marketTicker)
		tickers[symbol] = *ticker
	}

	if len(symbols) == 0 {
		return tickers, nil
	}

	selectedTickers := make(map[string]types.Ticker, len(symbols))
	for _, symbol := range symbols {
		if ticker, ok := tickers[symbol]; ok {
			selectedTickers[symbol] = ticker
		}
	}

	return selectedTickers, nil
}

func (e *Exchange) PlatformFeeCurrency() string {
	return PlatformToken
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	if e.IsFutures {
		return e.QueryFuturesAccount(ctx)
	}

	accounts, err := e.queryAccountBalance(ctx)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("account balance is empty")
	}

	accountConfigs, err := e.client.NewGetAccountConfigRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(accountConfigs) == 0 {
		return nil, fmt.Errorf("account config is empty")
	}

	balances := toGlobalBalance(&accounts[0])
	account := types.NewAccount()
	account.UpdateBalances(balances)

	account.TotalAccountValue = accounts[0].TotalEquityInUSD

	// For OKX margin account
	// OKX's margin ratio: Adjusted equity / (Maintenance margin + Liquidation fees)
	// OKX does not provide margin level, Margin Level = total asset value / total liabilities
	//
	// The forced liquidation of multi-currency margin mode is based on whether the maintenance margin ratio reaches 100%.
	// When the maintenance margin ratio is ≤ 300%, the system will send a warning to reduce the positions and the user should be aware of the liquidation risk. 300% is the warning parameter.
	// OKX reserves the right to adjust this parameter according to the actual situation.
	// When the maintenance margin ratio is ≤ 100%, the system will cancel orders according to the following rules, known as order cancellation by pre-liquidation:
	if e.MarginSettings.IsMargin {
		account.AccountType = types.AccountTypeMargin

		account.BorrowEnabled = types.BoolPtr(accountConfigs[0].EnableSpotBorrow)

		// Spot mode could have margin ratio as well
		account.MarginRatio = fixedpoint.NewFromFloat(1.0) // 100%

		if accounts[0].MarginRatio.Sign() > 0 {
			account.MarginRatio = accounts[0].MarginRatio
		}

		if accounts[0].NotionalUsdForBorrow.Sign() > 0 {
			account.MarginLevel = accounts[0].TotalEquityInUSD.Div(accounts[0].NotionalUsdForBorrow)
		} else {
			account.MarginLevel = fixedpoint.NewFromFloat(999.9)
		}

		if account.MarginLevel.Sign() > 0 {
			account.MarginTolerance = util.CalculateMarginTolerance(account.MarginLevel)
		}

		if !accountConfigs[0].EnableSpotBorrow {
			log.Warnf("margin is enabled, but okx enableSpotBorrow field is false, please turn on auto-borrow from the okx UI, this is the only way to enable spot margin auto-borrow")
		}
	}

	return account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	accountBalances, err := e.queryAccountBalance(ctx)
	if err != nil {
		return nil, err
	}

	if len(accountBalances) != 1 {
		return nil, fmt.Errorf("unexpected length of balances: %v", accountBalances)
	}

	return toGlobalBalance(&accountBalances[0]), nil
}

func (e *Exchange) queryAccountBalance(ctx context.Context) ([]okexapi.Account, error) {
	if err := queryAccountLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("account rate limiter wait error: %w", err)
	}

	return e.client.NewGetAccountBalanceRequest().Do(ctx)
}

/*
Market order behaviors under different account mode:

OKX Spot mode:
- Market order + TradeModeCash + TargetCurrencyBase ✅ (order history shows the order under "Spot")
- Market order + TradeModeCash + TargetCurrencyQuote ✅ (order history shows the order under "Spot")
- Market order + TradeModeCross + TargetCurrencyBase ❌ (You can't complete this request under your current account mode)
- Market order + TradeModeCross + TargetCurrencyQuote ❌ (You can't complete this request under your current account mode)
OKX Spot & Futures mode:
- Market order + TradeModeCash + TargetCurrencyBase ✅ (fallback to Spot mode, order history shows the order under "Spot")
- Market order + TradeModeCash + TargetCurrencyQuote ✅ (order history shows the order under "Spot")
- Market order + TradeModeCross + TargetCurrencyBase ❌ (The instrument corresponding to this BTC-USDT does not support the tgtCcy parameter)
- Market order + TradeModeCross + CurrencyQuote=USDT ✅ (order history shows the order under "Margin")

This affects the behavior of syncing order history.

Under OKX’s margin mode (specifically spot & futures mode), it’s possible to place orders using the cash mode.
However, if you try to query order history using the margin mode, you won’t be able to find those orders placed
via cash mode, because they’re categorized under “Spot”.

When using OKX margin, it seems that operating in Spot mode is generally easier—mainly because
market orders can specify size using the base currency instead of the quote.

That said, when syncing, we’ll need a workaround to handle this edge case: the query for order history
should be forced to use “spot” instead of “margin”.
*/
func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	req := e.client.NewPlaceOrderRequest()

	req.InstrumentID(e.getInstrumentId(order.Symbol))
	req.Side(toLocalSideType(order.Side))
	req.Size(order.Market.FormatQuantity(order.Quantity))

	if e.MarginSettings.IsMargin {
		// okx market order with trade mode cross will be rejected:
		//   "The corresponding product of this BTC-USDT doesn't support the tgtCcy parameter"
		// See: https://www.okx.com/docs-v5/trick_en/#order-management-trade-mode
		req.TradeMode(okexapi.TradeModeCash)
	} else if e.IsFutures {
		if e.FuturesSettings.IsIsolatedFutures {
			req.TradeMode(okexapi.TradeModeIsolated)
		} else {
			req.TradeMode(okexapi.TradeModeCross)
		}

		if dualSidePosition {
			setDualSidePosition(req, order)
		}
	} else {
		req.TradeMode(okexapi.TradeModeCash)
	}

	// set price field for limit orders
	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		req.Price(order.Market.FormatPrice(order.Price))
	case types.OrderTypeMarket:
		// target currency = Default is quote_ccy for buy, base_ccy for sell
		// Because our order.Quantity unit is base coin, so we indicate the target currency to Base.
		// Only applicable to SPOT Market Orders
		if !e.IsFutures {
			req.TargetCurrency(okexapi.TargetCurrencyBase)
		}
	}

	orderType, err := toLocalOrderType(order.Type)
	if err != nil {
		return nil, err
	}

	switch order.TimeInForce {
	case types.TimeInForceFOK:
		req.OrderType(okexapi.OrderTypeFOK)
	case types.TimeInForceIOC:
		req.OrderType(okexapi.OrderTypeIOC)
	default:
		req.OrderType(orderType)
	}

	if err := placeOrderLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("place order rate limiter wait error: %w", err)
	}

	if len(order.ClientOrderID) > 0 {
		if ok := clientOrderIdRegex.MatchString(order.ClientOrderID); !ok {
			return nil, fmt.Errorf("client order id should be case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters: %s", order.ClientOrderID)
		}
		req.ClientOrderID(order.ClientOrderID)
	}

	if len(e.brokerId) != 0 {
		req.Tag(e.brokerId)
	}

	timeNow := time.Now()
	orders, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(orders) != 1 {
		return nil, fmt.Errorf("unexpected length of order response: %v", orders)
	}

	orderID, err := strconv.ParseUint(orders[0].OrderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response order id: %w", err)
	}

	return &types.Order{
		SubmitOrder:      order,
		Exchange:         types.ExchangeOKEx,
		OrderID:          orderID,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: fixedpoint.Zero,
		IsWorking:        true,
		CreationTime:     types.Time(timeNow),
		UpdateTime:       types.Time(timeNow),
	}, nil
}

// QueryOpenOrders retrieves the pending orders. The data returned is ordered by createdTime, and we utilized the
// `After` parameter to acquire all orders.
func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	instrumentID := e.getInstrumentId(symbol)

	nextCursor := int64(0)
	for {
		if err := queryOpenOrderLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("query open orders rate limiter wait error: %w", err)
		}

		req := e.client.NewGetOpenOrdersRequest().
			InstrumentID(instrumentID).
			After(strconv.FormatInt(nextCursor, 10))

		req.InstrumentType(e.getInstrumentType())

		openOrders, err := req.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query open orders: %w", err)
		}

		for _, o := range openOrders {
			o, err := orderDetailToGlobalOrder(&o.OrderDetail)
			if err != nil {
				return nil, fmt.Errorf("failed to convert order, err: %v", err)
			}

			orders = append(orders, *o)
		}

		orderLen := len(openOrders)
		// a defensive programming to ensure the length of order response is expected.
		if orderLen > defaultQueryLimit {
			return nil, fmt.Errorf("unexpected open orders length %d", orderLen)
		}

		if orderLen < defaultQueryLimit {
			break
		}
		nextCursor = int64(openOrders[orderLen-1].OrderId)
	}

	return orders, err
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	if len(orders) == 0 {
		return nil
	}

	var reqs []*okexapi.CancelOrderRequest
	for _, order := range orders {
		if len(order.Symbol) == 0 {
			return ErrSymbolRequired
		}

		req := e.client.NewCancelOrderRequest()
		req.InstrumentID(e.getInstrumentId(order.Symbol))

		req.OrderID(strconv.FormatUint(order.OrderID, 10))
		if len(order.ClientOrderID) > 0 {
			if ok := clientOrderIdRegex.MatchString(order.ClientOrderID); !ok {
				return fmt.Errorf("client order id should be case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters: %s", order.ClientOrderID)
			}
			req.ClientOrderID(order.ClientOrderID)
		}
		reqs = append(reqs, req)
	}

	if err := batchCancelOrderLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("batch cancel order rate limiter wait error: %w", err)
	}
	batchReq := e.client.NewBatchCancelOrderRequest()
	batchReq.Add(reqs...)
	_, err := batchReq.Do(ctx)
	return err
}

func (e *Exchange) NewStream() types.Stream {
	s := NewStream(e.client, e)

	return s
}

func (e *Exchange) QueryKLines(
	ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions,
) ([]types.KLine, error) {
	if err := queryKLineLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query k line rate limiter wait error: %w", err)
	}

	intervalParam, err := toLocalInterval(interval)
	if err != nil {
		return nil, fmt.Errorf("failed to get interval: %w", err)
	}

	instrumentID := e.getInstrumentId(symbol)

	req := e.client.NewGetCandlesRequest().InstrumentID(instrumentID)
	req.Bar(intervalParam)

	if options.StartTime != nil {
		req.Before(*options.StartTime)
	}

	if options.EndTime != nil {
		req.After(*options.EndTime)
	}

	candles, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var klines []types.KLine
	for _, candle := range candles {
		klines = append(klines, kLineToGlobal(candle, interval, symbol))
	}

	return klines, nil

}

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	if len(q.Symbol) == 0 {
		return nil, ErrSymbolRequired
	}
	if len(q.OrderID) == 0 && len(q.ClientOrderID) == 0 {
		return nil, errors.New("okex.QueryOrder: OrderId or ClientOrderId is required parameter")
	}
	req := e.client.NewGetOrderDetailsRequest()
	instrumentID := e.getInstrumentId(q.Symbol)

	req.InstrumentID(instrumentID)
	// Either ordId or clOrdId is required, if both are passed, ordId will be used
	// ref: https://www.okx.com/docs-v5/en/#order-book-trading-trade-get-order-details
	if len(q.OrderID) != 0 {
		req.OrderID(q.OrderID)
	}
	// If the clOrdId is associated with multiple orders, only the latest one will be returned.
	if len(q.ClientOrderID) != 0 {
		req.ClientOrderID(q.ClientOrderID)
	}

	var order *okexapi.OrderDetails
	order, err := req.Do(ctx)

	if err != nil {
		return nil, err
	}

	return toGlobalOrder(order)
}

// QueryOrderTrades quires order trades can query trades in last 3 days.
func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) (trades []types.Trade, err error) {
	if len(q.ClientOrderID) != 0 {
		log.Warn("!!!OKEX EXCHANGE API NOTICE!!! Okex does not support searching for trades using OrderClientId.")
	}

	req := e.client.NewGetThreeDaysTransactionHistoryRequest()
	if len(q.Symbol) != 0 {
		instrumentID := e.getInstrumentId(q.Symbol)
		req.InstrumentID(instrumentID)
	}

	req.InstrumentType(e.getInstrumentType())

	if len(q.OrderID) != 0 {
		req.OrderID(q.OrderID)
	}

	response, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query order trades, err: %w", err)
	}

	for _, trade := range response {
		trades = append(trades, toGlobalTrade(trade))
	}

	return trades, nil
}

/*
QueryClosedOrders can query closed orders in last 3 months, there are no time interval limitations, as long as until >= since.
Please Use lastOrderID as cursor, only return orders later than that order, that order is not included.
If you want to query all orders within a large time range (e.g. total orders > 100), we recommend using batch.ClosedOrderBatchQuery.

** since and until are inclusive, you can include the lastTradeId as well. **
*/
func (e *Exchange) QueryClosedOrders(
	ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64,
) (orders []types.Order, err error) {
	if symbol == "" {
		return nil, ErrSymbolRequired
	}

	newSince := since
	now := time.Now()

	if time.Since(newSince) > maxHistoricalDataQueryPeriod {
		newSince = now.Add(-maxHistoricalDataQueryPeriod)
		log.Warnf("!!!OKX EXCHANGE API NOTICE!!! The closed order API cannot query data beyond 90 days from the current date, update %s -> %s", since, newSince)
	}
	if until.Before(newSince) {
		log.Warnf("!!!OKX EXCHANGE API NOTICE!!! The 'until' comes before 'since', update until to now(%s -> %s).", until, now)
		until = now
	}
	if until.Sub(newSince) > maxHistoricalDataQueryPeriod {
		return nil, fmt.Errorf("the start time %s and end time %s cannot exceed 90 days", newSince, until)
	}

	if err := queryClosedOrderRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query closed order rate limiter wait error: %w", err)
	}

	req := e.client.NewGetOrderHistoryRequest()
	instrumentID := e.getInstrumentId(symbol)

	req.InstrumentID(instrumentID).
		StartTime(since).
		EndTime(until).
		InstrumentType(e.getInstrumentType()).
		Limit(defaultQueryLimit).
		Before(strconv.FormatUint(lastOrderID, 10))

	res, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call get order histories error: %w", err)
	}

	for _, order := range res {
		o, err2 := orderDetailToGlobalOrder(&order)
		if err2 != nil {
			err = multierr.Append(err, err2)
			continue
		}

		orders = append(orders, *o)
	}
	if err != nil {
		return nil, err
	}

	return types.SortOrdersAscending(orders), nil
}

func (e *Exchange) RepayMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error {
	req := e.client.NewSpotManualBorrowRepayRequest()
	req.Currency(strings.ToUpper(asset))
	req.Amount(amount.String())
	req.Side(okexapi.MarginSideRepay)
	resp, err := req.Do(ctx)
	if err != nil {
		return err
	}

	log.Infof("spot repay response: %+v", resp)
	return nil
}

func (e *Exchange) BorrowMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error {
	req := e.client.NewSpotManualBorrowRepayRequest()
	req.Currency(strings.ToUpper(asset))
	req.Amount(amount.String())
	req.Side(okexapi.MarginSideBorrow)

	resp, err := req.Do(ctx)
	if err != nil {
		return err
	}

	log.Infof("spot borrow response: %+v", resp)
	return nil
}

func (e *Exchange) QueryMarginAssetMaxBorrowable(ctx context.Context, asset string) (fixedpoint.Value, error) {
	req := e.client.NewGetAccountMaxLoanRequest()
	req.Currency(asset).
		MarginMode(okexapi.MarginModeCross)

	resp, err := req.Do(ctx)
	if err != nil {
		return fixedpoint.Zero, err
	}

	if len(resp) == 0 {
		return fixedpoint.Zero, nil
	}

	return resp[0].MaxLoan, nil
}

func (e *Exchange) QueryLoanHistory(
	ctx context.Context, asset string, startTime, endTime *time.Time,
) ([]types.MarginLoan, error) {
	req := e.client.NewGetAccountSpotBorrowRepayHistoryRequest().Currency(asset)
	if endTime != nil {
		req.Before(*endTime)
	}
	if startTime != nil {
		req.After(*startTime)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var records []types.MarginLoan
	for _, r := range resp {
		switch r.Type {
		case okexapi.MarginEventTypeManualBorrow, okexapi.MarginEventTypeAutoBorrow:
			records = append(records, toGlobalMarginLoan(r))
		}
	}

	return records, nil
}

func (e *Exchange) QueryRepayHistory(
	ctx context.Context, asset string, startTime, endTime *time.Time,
) ([]types.MarginRepay, error) {
	req := e.client.NewGetAccountSpotBorrowRepayHistoryRequest().Currency(asset)
	if endTime != nil {
		req.Before(*endTime)
	}
	if startTime != nil {
		req.After(*startTime)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var records []types.MarginRepay
	for _, r := range resp {
		switch r.Type {
		case okexapi.MarginEventTypeManualRepay, okexapi.MarginEventTypeAutoRepay:
			records = append(records, toGlobalMarginRepay(r))
		}
	}

	return records, nil
}

func (e *Exchange) QueryLiquidationHistory(
	ctx context.Context, startTime, endTime *time.Time,
) ([]types.MarginLiquidation, error) {
	return nil, nil
}

func (e *Exchange) QueryInterestHistory(
	ctx context.Context, asset string, startTime, endTime *time.Time,
) ([]types.MarginInterest, error) {
	return nil, nil
}

func (e *Exchange) getInstrumentType() okexapi.InstrumentType {
	if e.MarginSettings.IsMargin {
		// Why do we set Spot here? See the doc comment of the SubmitOrder method
		return okexapi.InstrumentTypeSpot
	} else if e.IsFutures {
		return okexapi.InstrumentTypeSwap
	}

	return okexapi.InstrumentTypeSpot
}

func (e *Exchange) getInstrumentId(symbol string) string {
	if e.IsFutures {
		return toLocalSymbol(symbol, okexapi.InstrumentTypeSwap)
	}

	return toLocalSymbol(symbol)
}

func (e *Exchange) QueryDepositHistory(
	ctx context.Context, asset string, startTime, endTime *time.Time,
) ([]types.Deposit, error) {
	req := e.client.NewGetAssetDepositHistoryRequest().Currency(asset)
	if endTime != nil {
		req.Before(*endTime)
	}
	if startTime != nil {
		req.After(*startTime)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var records []types.Deposit
	for _, r := range resp {
		records = append(records, toGlobalDeposit(r))
	}

	return records, nil
}

/*
QueryTrades can query trades in last 3 months, there are no time interval limitations, as long as end_time >= start_time.
okx does not provide an API to query by trade ID, so we use the bill ID to do it. The trades result is ordered by timestamp.

REMARK: If your start time is 90 days earlier, we will update it to now - 90 days.
** StartTime and EndTime are inclusive. **
** StartTime and EndTime cannot exceed 90 days.  **
** StartTime, EndTime, FromTradeId can be used together. **

If you want to query all trades within a large time range (e.g. total orders > 100), we recommend using batch.TradeBatchQuery.
We don't support the last trade id as a filter because okx supports bill ID only.
*/
func (e *Exchange) QueryTrades(
	ctx context.Context, symbol string, options *types.TradeQueryOptions,
) (trades []types.Trade, err error) {
	logger := util.GetLoggerFromCtxOrFallback(ctx, log)
	if symbol == "" {
		return nil, ErrSymbolRequired
	}

	limit := options.Limit
	if limit > defaultQueryLimit || limit <= 0 {
		logger.Debugf("param limit exceeded default limit %d or zero, got: %d, use default limit", defaultQueryLimit, limit)
		limit = defaultQueryLimit
	}

	timeNow := e.timeNowFunc()
	newStartTime := timeNow.Add(-threeDaysHistoricalPeriod)
	if options.StartTime != nil {
		newStartTime = *options.StartTime
		if timeNow.Sub(newStartTime) > maxHistoricalDataQueryPeriod {
			newStartTime = timeNow.Add(-maxHistoricalDataQueryPeriod)
			logger.Warnf("!!!OKX EXCHANGE API NOTICE!!! The trade API cannot query data beyond 90 days from the current date, update %s -> %s", *options.StartTime, newStartTime)
		}
	}

	endTime := timeNow
	if options.EndTime != nil {
		if options.EndTime.Before(newStartTime) {
			return nil, fmt.Errorf("end time %s before start %s", *options.EndTime, newStartTime)
		}
		if options.EndTime.Sub(newStartTime) > maxHistoricalDataQueryPeriod {
			return nil, fmt.Errorf("start time %s and end time %s cannot greater than 90 days", newStartTime, options.EndTime)
		}
		endTime = *options.EndTime
	}

	if options.LastTradeID != 0 {
		// we don't support the last trade id as a filter because okx supports bill ID only.
		// we don't have any more fields (types.Trade) to store it.
		logger.Debugf("Last trade id: %d not supported on QueryTrades", options.LastTradeID)
	}

	lessThan3Day := timeNow.Sub(newStartTime) <= threeDaysHistoricalPeriod

	logger = logger.WithFields(logrus.Fields{
		"symbol":        symbol,
		"limit":         limit,
		"start_time":    newStartTime,
		"end_time":      endTime,
		"last_trade_id": options.LastTradeID,
		"less_3day":     lessThan3Day,
		"margin":        e.MarginSettings.IsMargin,
		"req_id":        uuid.New().String(),
	})

	instrumentID := e.getInstrumentId(symbol)
	instrType := e.getInstrumentType()

	if lessThan3Day {
		req := e.client.NewGetThreeDaysTransactionHistoryRequest().
			InstrumentID(instrumentID).
			InstrumentType(instrType).
			StartTime(newStartTime).
			EndTime(endTime).
			Limit(uint64(limit))

		return getTrades(ctx, logger, limit, func(ctx context.Context, billId string) ([]okexapi.Trade, error) {
			if billId != "" && billId != "0" {
				// the `after` will get the data after the billId
				// so DON'T fill the billId if it's 0
				req.After(billId)
			}
			return req.Do(ctx)
		})
	}

	req := e.client.NewGetTransactionHistoryRequest().
		InstrumentID(instrumentID).
		InstrumentType(instrType).
		StartTime(newStartTime).
		EndTime(endTime).
		Limit(uint64(limit))

	return getTrades(ctx, logger, limit, func(ctx context.Context, billId string) ([]okexapi.Trade, error) {
		if billId != "" && billId != "0" {
			// the `after` will get the data after the billId
			// so DON'T fill the billId if it's 0
			req.After(billId)
		}

		return req.Do(ctx)
	})
}

func getTrades(
	ctx context.Context, logger *logrus.Entry, limit int64,
	doFunc func(ctx context.Context, billId string) ([]okexapi.Trade, error),
) (trades []types.Trade, err error) {
	billId := "0"
	for {
		response, err := doFunc(ctx, billId)
		if err != nil {
			logger.WithError(err).Warn("failed to query trades")
			return nil, fmt.Errorf("failed to query trades, err: %w", err)
		}

		for _, trade := range response {
			trades = append(trades, toGlobalTrade(trade))
		}

		tradeLen := int64(len(response))
		// a defensive programming to ensure the length of order response is expected.
		if tradeLen > limit {
			logger.WithError(err).Warnf("getTrades: trade length %d exceeds limit %d", tradeLen, limit)
			return nil, fmt.Errorf("unexpected trade length %d", tradeLen)
		} else if tradeLen < limit {
			logger.Warnf("getTrades: trade length %d less than limit %d", tradeLen, limit)
			break
		}

		// use After filter to get all data.
		billId = response[tradeLen-1].BillId.String()
	}

	return trades, nil
}

func (e *Exchange) SupportedInterval() map[types.Interval]int {
	return SupportedIntervals
}

func (e *Exchange) IsSupportedInterval(interval types.Interval) bool {
	_, ok := SupportedIntervals[interval]
	return ok
}

func (e *Exchange) GetClient() *okexapi.RestClient {
	return e.client
}

// syncLocalSymbolMap synchronizes the local symbol map with market data:
// - Updates existing symbols
// - Adds new symbols from markets
// - Removes symbols that no longer exist in markets
func (e *Exchange) syncLocalSymbolMap(markets types.MarketMap) {
	symbolMap := spotSymbolSyncMap
	if e.IsFutures {
		symbolMap = swapSymbolSyncMap
	}

	existingSymbols := make(map[string]struct{})

	// Update existing and add new symbols
	for symbol, market := range markets {
		symbolMap.Store(symbol, market.LocalSymbol)
		existingSymbols[symbol] = struct{}{}
	}

	// Remove symbols that don't exist in markets anymore
	symbolMap.Range(func(key, value interface{}) bool {
		symbol, ok := key.(string)
		if !ok {
			return true
		}
		if _, exists := existingSymbols[symbol]; !exists {
			symbolMap.Delete(symbol)
		}
		return true
	})
}

func setDualSidePosition(req *okexapi.PlaceOrderRequest, order types.SubmitOrder) {
	switch order.Side {
	case types.SideTypeBuy:
		if order.ReduceOnly {
			req.PosSide(okexapi.PosSideShort).ReduceOnly(true)
		} else {
			req.PosSide(okexapi.PosSideLong)
		}
	case types.SideTypeSell:
		if order.ReduceOnly {
			req.PosSide(okexapi.PosSideLong).ReduceOnly(true)
		} else {
			req.PosSide(okexapi.PosSideShort)
		}
	}
}
