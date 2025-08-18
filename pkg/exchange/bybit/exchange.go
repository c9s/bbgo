package bybit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	maxOrderIdLen          = 36
	defaultQueryLimit      = 50
	defaultQueryTradeLimit = 100
	defaultKLineLimit      = 1000

	queryTradeDurationLimit = 7 * 24 * time.Hour

	maxHistoricalDataQueryPeriod = 2 * 365 * 24 * time.Hour
)

// https://bybit-exchange.github.io/docs/zh-TW/v5/rate-limit
// GET/POST method (shared): 120 requests per second for 5 consecutive seconds
var (
	// sharedRateLimiter indicates that the API belongs to the public API.
	// The default order limiter apply 5 requests per second and a 5 initial bucket
	// this includes QueryMarkets, QueryTicker, QueryAccountBalances, GetFeeRates
	sharedRateLimiter = rate.NewLimiter(rate.Every(time.Second/5), 5)

	log = logrus.WithFields(logrus.Fields{
		"exchange": "bybit",
	})

	_ types.ExchangeAccountService    = &Exchange{}
	_ types.ExchangeMarketDataService = &Exchange{}
	_ types.CustomIntervalProvider    = &Exchange{}
	_ types.ExchangeMinimal           = &Exchange{}
	_ types.ExchangeTradeService      = &Exchange{}
	_ types.Exchange                  = &Exchange{}
	_ types.ExchangeOrderQueryService = &Exchange{}
)

type Exchange struct {
	key, secret string
	client      *bybitapi.RestClient
	marketsInfo types.MarketMap

	// feeRateProvider provides the fee rate and fee currency for each symbol.
	// Because the bybit exchange does not provide a fee currency on traditional SPOT accounts, we need to query the marker
	// fee rate to get the fee currency.
	// https://bybit-exchange.github.io/docs/v5/enum#spot-fee-currency-instruction
	FeeRatePoller
}

func New(key, secret string) (*Exchange, error) {
	client, err := bybitapi.NewClient()
	if err != nil {
		return nil, err
	}
	ex := &Exchange{
		key: key,
		// pragma: allowlist nextline secret
		secret: secret,
		client: client,
	}
	if len(key) > 0 && len(secret) > 0 {
		client.Auth(key, secret)
		ex.FeeRatePoller = newFeeRatePoller(ex)

		ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("query markets timeout"))
		defer cancel()
		ex.marketsInfo, err = ex.QueryMarkets(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query markets, err: %w", err)
		}
	}

	return ex, nil
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBybit
}

// PlatformFeeCurrency returns empty string. The platform does not support "PlatformFeeCurrency" but instead charges
// fees using the native token.
func (e *Exchange) PlatformFeeCurrency() string {
	return ""
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	if err := sharedRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("markets rate limiter wait error: %w", err)
	}

	instruments, err := e.client.NewGetInstrumentsInfoRequest().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get instruments, err: %v", err)
	}

	marketMap := types.MarketMap{}
	for _, s := range instruments.List {
		marketMap.Add(toGlobalMarket(s))
	}

	return marketMap, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	if err := sharedRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("ticker order rate limiter wait error: %w", err)
	}

	s, err := e.client.NewGetTickersRequest().Symbol(symbol).DoWithResponseTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call ticker, symbol: %s, err: %w", symbol, err)
	}

	if len(s.List) != 1 {
		return nil, fmt.Errorf("unexpected ticker length, exp:1, got:%d", len(s.List))
	}

	ticker := toGlobalTicker(s.List[0], s.ClosedTime.Time())
	return &ticker, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbols ...string) (map[string]types.Ticker, error) {
	tickers := map[string]types.Ticker{}
	if len(symbols) > 0 {
		for _, s := range symbols {
			t, err := e.QueryTicker(ctx, s)
			if err != nil {
				return nil, err
			}

			tickers[s] = *t
		}

		return tickers, nil
	}

	if err := sharedRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("tickers rate limiter wait error: %w", err)
	}
	allTickers, err := e.client.NewGetTickersRequest().DoWithResponseTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call ticker, err: %w", err)
	}

	for _, s := range allTickers.List {
		tickers[s.Symbol] = toGlobalTicker(s, allTickers.ClosedTime.Time())
	}

	return tickers, nil
}

// QueryOpenOrders queries open orders by symbol.
//
// Primarily query unfilled or partially filled orders in real-time, but also supports querying recent 500 closed status
// (Cancelled, Filled) orders. Please see the usage of request param openOnly.
// UTA2.0 can query filled, canceled, and rejected orders to the most recent 500 orders for spot, linear, inverse and
// option categories
//
// The records are sorted by the createdTime from newest to oldest.
func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	cursor := ""
	// OpenOnlyOrder: UTA2.0, UTA1.0, classic account query open status orders (e.g., New, PartiallyFilled) only
	req := e.client.NewGetOpenOrderRequest().Symbol(symbol).OpenOnly(bybitapi.OpenOnlyOrder).Limit(defaultQueryLimit)
	for {
		if len(cursor) != 0 {
			// the default limit is 20.
			req = req.Cursor(cursor)
		}

		res, err := req.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query open orders, err: %w", err)
		}

		for _, order := range res.List {
			order, err := toGlobalOrder(order)
			if err != nil {
				return nil, fmt.Errorf("failed to convert order, err: %v", err)
			}

			orders = append(orders, *order)
		}

		if len(res.NextPageCursor) == 0 {
			break
		}
		cursor = res.NextPageCursor
	}

	return orders, nil
}

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	if len(q.OrderID) == 0 && len(q.ClientOrderID) == 0 {
		return nil, errors.New("one of OrderID/ClientOrderID is required parameter")
	}

	if len(q.OrderID) != 0 && len(q.ClientOrderID) != 0 {
		return nil, errors.New("only accept one parameter of OrderID/ClientOrderID")
	}

	req := e.client.NewGetOrderHistoriesRequest()
	if len(q.Symbol) != 0 {
		req.Symbol(q.Symbol)
	}

	if len(q.OrderID) != 0 {
		req.OrderId(q.OrderID)
	}

	if len(q.ClientOrderID) != 0 {
		req.OrderLinkId(q.ClientOrderID)
	}

	res, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query order, queryConfig: %+v, err: %w", q, err)
	}
	if len(res.List) == 0 {
		return nil, fmt.Errorf("order not found, queryConfig: %+v", q)
	}
	if len(res.List) != 1 {
		return nil, fmt.Errorf("unexpected order histories length: %d, queryConfig: %+v", len(res.List), q)
	}

	return toGlobalOrder(res.List[0])
}

// QueryOrderTrades You can query by symbol, baseCoin, orderId and orderLinkId, and if you pass multiple params,
// the system will process them according to this priority: orderId > orderLinkId > symbol > baseCoin.
func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) (trades []types.Trade, err error) {
	req := e.client.NewGetExecutionListRequest()
	if len(q.ClientOrderID) != 0 {
		req.OrderLinkId(q.ClientOrderID)
	}

	if len(q.OrderID) != 0 {
		req.OrderLinkId(q.OrderID)
	}

	if len(q.Symbol) != 0 {
		req.Symbol(q.Symbol)
	}
	req.Limit(defaultQueryTradeLimit)

	return e.queryTrades(ctx, req)
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	if len(order.Market.Symbol) == 0 {
		return nil, fmt.Errorf("order.Market.Symbol is required: %+v", order)
	}

	req := e.client.NewPlaceOrderRequest()
	req.Symbol(order.Market.Symbol)

	// set order type
	orderType, err := toLocalOrderType(order.Type)
	if err != nil {
		return nil, err
	}
	req.OrderType(orderType)

	// set side
	side, err := toLocalSide(order.Side)
	if err != nil {
		return nil, err
	}
	req.Side(side)
	req.Qty(order.Market.FormatQuantity(order.Quantity))

	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		req.Price(order.Market.FormatPrice(order.Price))
	case types.OrderTypeMarket:
		// Because our order.Quantity unit is base coin, so we indicate the target currency to Base.
		if order.Side == types.SideTypeBuy {
			req.MarketUnit(bybitapi.MarketUnitBase)
		} else {
			req.MarketUnit(bybitapi.MarketUnitQuote)
		}
	}

	// set timeInForce
	switch order.TimeInForce {
	case types.TimeInForceFOK:
		req.TimeInForce(bybitapi.TimeInForceFOK)
	case types.TimeInForceIOC:
		req.TimeInForce(bybitapi.TimeInForceIOC)
	default:
		req.TimeInForce(bybitapi.TimeInForceGTC)
	}

	// set client order id
	if len(order.ClientOrderID) > maxOrderIdLen {
		return nil, fmt.Errorf("unexpected length of order id, got: %d", len(order.ClientOrderID))
	}
	if len(order.ClientOrderID) > 0 {
		req.OrderLinkId(order.ClientOrderID)
	}

	timeNow := time.Now()
	res, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to place order, order: %#v, err: %w", order, err)
	}

	if len(res.OrderId) == 0 || (len(order.ClientOrderID) != 0 && res.OrderLinkId != order.ClientOrderID) {
		return nil, fmt.Errorf("unexpected order id, resp: %#v, order: %#v", res, order)
	}

	intOrderId, err := strconv.ParseUint(res.OrderId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse orderId: %s", res.OrderId)
	}

	return &types.Order{
		SubmitOrder:      order,
		Exchange:         types.ExchangeBybit,
		OrderID:          intOrderId,
		UUID:             res.OrderId,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: fixedpoint.Zero,
		IsWorking:        true,
		CreationTime:     types.Time(timeNow),
		UpdateTime:       types.Time(timeNow),
	}, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (errs error) {
	if len(orders) == 0 {
		return nil
	}

	for _, order := range orders {
		req := e.client.NewCancelOrderRequest()

		reqId := ""
		switch {
		// use the OrderID first, then the ClientOrderID
		case order.OrderID > 0:
			req.OrderId(order.UUID)
			reqId = order.UUID

		case len(order.ClientOrderID) != 0:
			req.OrderLinkId(order.ClientOrderID)
			reqId = order.ClientOrderID

		default:
			errs = multierr.Append(
				errs,
				fmt.Errorf("the order uuid and client order id are empty, order: %#v", order),
			)
			continue
		}

		req.Symbol(order.Market.Symbol)

		res, err := req.Do(ctx)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to cancel order id: %s, err: %w", order.ClientOrderID, err))
			continue
		}

		// sanity check
		if res.OrderId != reqId && res.OrderLinkId != reqId {
			errs = multierr.Append(errs, fmt.Errorf("order id mismatch, exp: %s, respOrderId: %s, respOrderLinkId: %s", reqId, res.OrderId, res.OrderLinkId))
			continue
		}
	}

	return errs
}

// QueryClosedOrders queries closed orders by symbol, since, until, and lastOrderID.
// startTime and endTime are not passed, return 7 days by default
// Only startTime is passed, return range between startTime and startTime+7 days
// Only endTime is passed, return range between endTime-7 days and endTime
// If both are passed, the rule is endTime - startTime <= 7 days
//
// ** since and until are inclusive. **
// ** sort by creation time in descending order. **
func (e *Exchange) QueryClosedOrders(
	ctx context.Context, symbol string, since, until time.Time, _ uint64,
) (orders []types.Order, err error) {

	now := time.Now()

	if time.Since(since) > maxHistoricalDataQueryPeriod {
		newSince := now.Add(-maxHistoricalDataQueryPeriod)
		log.Warnf("!!!BYBIT EXCHANGE API NOTICE!!! The closed order API cannot query data beyond 2 years from the current date, update %s -> %s", since, newSince)
		since = newSince
	}
	if until.Before(since) {
		newUntil := since.Add(queryTradeDurationLimit)
		log.Warnf("!!!BYBIT EXCHANGE API NOTICE!!! The 'until' comes before 'since', add 7 days to until (%s -> %s).", until, newUntil)
		until = newUntil
	}

	// if the time range exceeds the server boundary, get the last 7 days of data
	if until.Sub(since) > queryTradeDurationLimit {
		newStartTime := until.Add(-queryTradeDurationLimit)

		log.Warnf("!!!BYBIT EXCHANGE API NOTICE!!! The time range exceeds the server boundary: %s, start time: %s, end time: %s, updated start time %s -> %s", queryTradeDurationLimit, since.String(), until.String(), since.String(), newStartTime.String())
		since = newStartTime
	}
	req := e.client.NewGetOrderHistoriesRequest().
		Symbol(symbol).
		Limit(defaultQueryLimit).
		StartTime(since).
		EndTime(until)

	cursor := ""
	for {
		if len(cursor) != 0 {
			req = req.Cursor(cursor)
		}

		res, err := req.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to call get order histories error: %w", err)
		}

		for _, order := range res.List {
			order, err := toGlobalOrder(order)
			if err != nil {
				return nil, fmt.Errorf("failed to convert order, err: %v", err)
			}

			orders = append(orders, *order)
		}

		if len(res.NextPageCursor) == 0 {
			break
		}
		cursor = res.NextPageCursor
	}

	return types.SortOrdersAscending(orders), nil
}

func (e *Exchange) queryTrades(ctx context.Context, req *bybitapi.GetExecutionListRequest) (trades []types.Trade, err error) {
	cursor := ""
	for {
		if len(cursor) != 0 {
			req = req.Cursor(cursor)
		}

		res, err := req.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query trades, err: %w", err)
		}

		for _, trade := range res.List {
			trade, err := toGlobalTrade(trade)
			if err != nil {
				return nil, fmt.Errorf("failed to convert trade, err: %v", err)
			}

			trades = append(trades, *trade)
		}

		if len(res.NextPageCursor) == 0 {
			break
		}
		cursor = res.NextPageCursor
	}

	return trades, nil

}

/*
QueryTrades queries trades by time range.
** startTime and endTime are not passed, return 7 days by default **
** Only startTime is passed, return range between startTime and startTime+7 days **
** Only endTime is passed, return range between endTime-7 days and endTime **
** If both are passed, the rule is endTime - startTime <= 7 days **
*/
func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	req := e.client.NewGetExecutionListRequest()
	req.Symbol(symbol)

	if options.StartTime != nil && options.EndTime != nil {
		if options.EndTime.Before(*options.StartTime) {
			return nil, fmt.Errorf("end time is before start time, start time: %s, end time: %s", options.StartTime.String(), options.EndTime.String())
		}

		if options.EndTime.Sub(*options.StartTime) > queryTradeDurationLimit {
			newStartTime := options.EndTime.Add(-queryTradeDurationLimit)

			log.Warnf("!!!BYBIT EXCHANGE API NOTICE!!! The time range exceeds the server boundary: %s, start time: %s, end time: %s, updated start time %s -> %s", queryTradeDurationLimit, options.StartTime.String(), options.EndTime.String(), options.StartTime.String(), newStartTime.String())
			options.StartTime = &newStartTime
		}
	}

	if options.StartTime != nil {
		req.StartTime(options.StartTime.UTC())
	}
	if options.EndTime != nil {
		req.EndTime(options.EndTime.UTC())
	}

	limit := uint64(options.Limit)
	if limit > defaultQueryTradeLimit || limit <= 0 {
		log.Debugf("the parameter limit exceeds the server boundary or is set to zero. changed to %d, original value: %d", defaultQueryTradeLimit, options.Limit)
		limit = defaultQueryTradeLimit
	}
	req.Limit(limit)

	return e.queryTrades(ctx, req)
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	balanceMap, err := e.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}
	acct := &types.Account{
		AccountType: types.AccountTypeSpot,
		// MakerFeeRate bybit doesn't support global maker fee rate.
		MakerFeeRate: fixedpoint.Zero,
		// TakerFeeRate bybit doesn't support global taker fee rate.
		TakerFeeRate: fixedpoint.Zero,
	}
	acct.UpdateBalances(balanceMap)

	return acct, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	req := e.client.NewGetWalletBalancesRequest()
	accounts, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalBalanceMap(accounts.List), nil
}

/*
QueryKLines queries for historical klines (also known as candles/candlesticks). Charts are returned in groups based
on the requested interval.

A k-line's start time is inclusive, but end time is not(startTime + interval - 1 millisecond).
e.q. 15m interval k line can be represented as 00:00:00.000 ~ 00:14:59.999
*/
func (e *Exchange) QueryKLines(
	ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions,
) ([]types.KLine, error) {
	req := e.client.NewGetKLinesRequest().Symbol(symbol)
	intervalStr, err := toLocalInterval(interval)
	if err != nil {
		return nil, err
	}
	req.Interval(intervalStr)

	limit := uint64(options.Limit)
	if limit > defaultKLineLimit || limit <= 0 {
		log.Debugf("the parameter limit exceeds the server boundary or is set to zero. changed to %d, original value: %d", defaultQueryLimit, options.Limit)
		limit = defaultKLineLimit
	}
	req.Limit(limit)

	if options.StartTime != nil {
		req.StartTime(*options.StartTime)
	}

	if options.EndTime != nil {
		req.EndTime(*options.EndTime)
	}

	if err := sharedRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query klines rate limiter wait error: %w", err)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call k line, err: %w", err)
	}

	if resp.Category != bybitapi.CategorySpot {
		return nil, fmt.Errorf("unexpected category: %s", resp.Category)
	}

	if resp.Symbol != symbol {
		return nil, fmt.Errorf("unexpected symbol: %s, exp: %s", resp.Category, symbol)
	}

	kLines := toGlobalKLines(symbol, interval, resp.List)
	return types.SortKLinesAscending(kLines), nil

}

func (e *Exchange) SupportedInterval() map[types.Interval]int {
	return bybitapi.SupportedIntervals
}

func (e *Exchange) IsSupportedInterval(interval types.Interval) bool {
	_, ok := bybitapi.SupportedIntervals[interval]
	return ok
}

func (e *Exchange) GetAllFeeRates(ctx context.Context) (bybitapi.FeeRates, error) {
	if err := sharedRateLimiter.Wait(ctx); err != nil {
		return bybitapi.FeeRates{}, fmt.Errorf("query fee rate limiter wait error: %w", err)
	}
	feeRates, err := e.client.NewGetFeeRatesRequest().Do(ctx)
	if err != nil {
		return bybitapi.FeeRates{}, fmt.Errorf("failed to get fee rates, err: %w", err)
	}

	return *feeRates, nil
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.key, e.secret, e)
}
