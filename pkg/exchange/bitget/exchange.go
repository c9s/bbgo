package bitget

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	v2 "github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi/v2"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	ID = "bitget"

	PlatformToken = "BGB"

	queryLimit                   = 100
	defaultKLineLimit            = 100
	maxOrderIdLen                = 36
	maxHistoricalDataQueryPeriod = 90 * 24 * time.Hour
)

var log = logrus.WithFields(logrus.Fields{
	"exchange": ID,
})

var (
	// queryMarketRateLimiter has its own rate limit. https://bitgetlimited.github.io/apidoc/en/spot/#get-symbols
	queryMarketRateLimiter = rate.NewLimiter(rate.Every(time.Second/10), 5)

	// queryAccountRateLimiter has its own rate limit. https://bitgetlimited.github.io/apidoc/en/spot/#get-account-assets
	queryAccountRateLimiter = rate.NewLimiter(rate.Every(time.Second/5), 5)

	// queryTickerRateLimiter has its own rate limit. https://bitgetlimited.github.io/apidoc/en/spot/#get-single-ticker
	queryTickerRateLimiter = rate.NewLimiter(rate.Every(time.Second/10), 5)

	// queryTickersRateLimiter has its own rate limit. https://bitgetlimited.github.io/apidoc/en/spot/#get-all-tickers
	queryTickersRateLimiter = rate.NewLimiter(rate.Every(time.Second/10), 5)

	// queryOpenOrdersRateLimiter has its own rate limit. https://www.bitget.com/zh-CN/api-doc/spot/trade/Get-Unfilled-Orders
	queryOpenOrdersRateLimiter = rate.NewLimiter(rate.Every(time.Second/10), 5)

	// closedQueryOrdersRateLimiter has its own rate limit. https://www.bitget.com/api-doc/spot/trade/Get-History-Orders
	closedQueryOrdersRateLimiter = rate.NewLimiter(rate.Every(time.Second/15), 5)

	// submitOrderRateLimiter has its own rate limit. https://www.bitget.com/zh-CN/api-doc/spot/trade/Place-Order
	submitOrderRateLimiter = rate.NewLimiter(rate.Every(time.Second/5), 5)

	// queryTradeRateLimiter has its own rate limit. https://www.bitget.com/zh-CN/api-doc/spot/trade/Get-Fills
	queryTradeRateLimiter = rate.NewLimiter(rate.Every(time.Second/5), 5)

	// cancelOrderRateLimiter has its own rate limit. https://www.bitget.com/api-doc/spot/trade/Cancel-Order
	cancelOrderRateLimiter = rate.NewLimiter(rate.Every(time.Second/5), 5)

	// kLineRateLimiter has its own rate limit. https://www.bitget.com/api-doc/spot/market/Get-Candle-Data
	kLineRateLimiter = rate.NewLimiter(rate.Every(time.Second/10), 5)
)

type Exchange struct {
	key, secret, passphrase string

	client    *bitgetapi.RestClient
	v2client  *v2.Client
	timeNowFn func() time.Time
}

func New(key, secret, passphrase string) *Exchange {
	client := bitgetapi.NewClient()

	if len(key) > 0 && len(secret) > 0 {
		client.Auth(key, secret, passphrase)
	}

	return &Exchange{
		key:        key,
		secret:     secret,
		passphrase: passphrase,
		client:     client,
		v2client:   v2.NewClient(client),
		timeNowFn: func() time.Time {
			return time.Now()
		},
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBitget
}

func (e *Exchange) PlatformFeeCurrency() string {
	return PlatformToken
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.key, e.secret, e.passphrase)
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	if err := queryMarketRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("markets rate limiter wait error: %w", err)
	}

	req := e.v2client.NewGetSymbolsRequest()
	symbols, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, s := range symbols {
		if s.Status == v2.SymbolStatusOffline {
			// ignore offline symbols
			continue
		}

		markets[s.Symbol] = toGlobalMarket(s)
	}

	return markets, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	if err := queryTickerRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("ticker rate limiter wait error: %w", err)
	}

	req := e.v2client.NewGetTickersRequest()
	req.Symbol(symbol)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query ticker, symbol: %s, err: %w", symbol, err)
	}
	if len(resp) != 1 {
		return nil, fmt.Errorf("unexpected length of query single symbol: %s, resp: %+v", symbol, resp)
	}

	ticker := toGlobalTicker(resp[0])
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

	if err := queryTickersRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("tickers rate limiter wait error: %w", err)
	}

	resp, err := e.v2client.NewGetTickersRequest().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query tickers: %w", err)
	}

	for _, s := range resp {
		tickers[s.Symbol] = toGlobalTicker(s)
	}

	return tickers, nil
}

// QueryKLines queries the k line data by interval and time range...etc.
//
// If you provide only the start time, the system will return the latest data.
// If you provide both the start and end times, the system will return data within the specified range.
// If you provide only the end time, the system will return data that occurred before the end time.
//
// The end time has different limits. 1m, 5m can query for one month,15m can query for 52 days,30m can query for 62 days,
// 1H can query for 83 days,4H can query for 240 days,6H can query for 360 days.
func (e *Exchange) QueryKLines(
	ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions,
) ([]types.KLine, error) {
	req := e.v2client.NewGetKLineRequest().Symbol(symbol)
	intervalStr, found := toLocalGranularity[interval]
	if !found {
		return nil, fmt.Errorf("%s not supported, supported granlarity: %+v", intervalStr, toLocalGranularity)
	}
	req.Granularity(intervalStr)

	limit := uint64(options.Limit)
	if limit > defaultKLineLimit || limit <= 0 {
		log.Debugf("limtit is exceeded or zero, update to %d, got: %d", defaultKLineLimit, options.Limit)
		limit = defaultKLineLimit
	}
	req.Limit(strconv.FormatUint(limit, 10))

	if options.StartTime != nil {
		req.StartTime(*options.StartTime)
	}

	if options.EndTime != nil {
		if options.StartTime != nil && options.EndTime.Before(*options.StartTime) {
			return nil, fmt.Errorf("end time %s before start time %s", *options.EndTime, *options.StartTime)
		}

		ok, duration := hasMaxDuration(interval)
		if ok && time.Since(*options.EndTime) > duration {
			return nil, fmt.Errorf("end time %s are greater than max duration %s", *options.EndTime, duration)
		}
		req.EndTime(*options.EndTime)
	}

	if err := kLineRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query klines rate limiter wait error: %w", err)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call k line, err: %w", err)
	}

	kLines := toGlobalKLines(symbol, interval, resp)
	return types.SortKLinesAscending(kLines), nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	bals, err := e.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}

	account := types.NewAccount()
	account.UpdateBalances(bals)
	return account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	if err := queryAccountRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("account rate limiter wait error: %w", err)
	}

	req := e.v2client.NewGetAccountAssetsRequest().AssetType(v2.AssetTypeHoldOnly)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query account assets: %w", err)
	}

	bals := types.BalanceMap{}
	for _, asset := range resp {
		b := toGlobalBalance(asset)
		bals[asset.Coin] = b
	}

	return bals, nil
}

// SubmitOrder submits an order.
//
// Remark:
// 1. We support only GTC for time-in-force, because the response from queryOrder does not include time-in-force information.
// 2. For market buy orders, the size unit is quote currency, whereas the unit for order.Quantity is in base currency.
// Therefore, we need to calculate the equivalent quote currency amount based on the ticker data.
//
// Note that there is a bug in Bitget where you can place a market order with the 'post_only' option successfully,
// which should not be possible. The issue has been reported.
func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	if len(order.Market.Symbol) == 0 {
		return nil, fmt.Errorf("order.Market.Symbol is required: %+v", order)
	}

	req := e.v2client.NewPlaceOrderRequest()
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

	// set quantity
	qty := order.Quantity
	// if the order is market buy, the quantity is quote coin, instead of base coin. so we need to convert it.
	if order.Type == types.OrderTypeMarket && order.Side == types.SideTypeBuy {
		ticker, err := e.QueryTicker(ctx, order.Market.Symbol)
		if err != nil {
			return nil, err
		}
		qty = order.Quantity.Mul(ticker.Buy)
	}

	req.Size(order.Market.FormatQuantity(qty))

	// set TimeInForce
	// we only support GTC/PostOnly, because:
	// 1. we only support SPOT trading.
	// 2. The query open/closed order does not include the `force` in SPOT.
	// If we support FOK/IOC, but you can't query them, that would be unreasonable.
	// The other case to consider is 'PostOnly', which is a trade-off because we want to support 'xmaker'.
	if len(order.TimeInForce) != 0 && order.TimeInForce != types.TimeInForceGTC {
		return nil, fmt.Errorf("time-in-force %s not supported", order.TimeInForce)
	}

	switch order.Type {
	case types.OrderTypeLimitMaker:
		req.Force(v2.OrderForcePostOnly)
	default:
		req.Force(v2.OrderForceGTC)
	}

	// set price
	switch order.Type {
	case types.OrderTypeLimit, types.OrderTypeLimitMaker:
		req.Price(order.Market.FormatPrice(order.Price))

	}

	// set client order id
	if len(order.ClientOrderID) > maxOrderIdLen {
		return nil, fmt.Errorf("unexpected length of client order id, got: %d", len(order.ClientOrderID))
	}

	if len(order.ClientOrderID) > 0 {
		req.ClientOrderId(order.ClientOrderID)
	}

	if err := submitOrderRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("place order rate limiter wait error: %w", err)
	}

	timeNow := time.Now()
	res, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to place order, order: %#v, err: %w", order, err)
	}

	debugf("order created: %+v", res)

	if len(res.OrderId) == 0 || (len(order.ClientOrderID) != 0 && res.ClientOrderId != order.ClientOrderID) {
		return nil, fmt.Errorf("unexpected order id, resp: %#v, order: %#v", res, order)
	}

	intOrderId, err := strconv.ParseUint(res.OrderId, 10, 64)
	if err != nil {
		return nil, err
	}

	return &types.Order{
		SubmitOrder:      order,
		Exchange:         types.ExchangeBitget,
		OrderID:          intOrderId,
		UUID:             res.OrderId,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: fixedpoint.Zero,
		IsWorking:        true,
		CreationTime:     types.Time(timeNow),
		UpdateTime:       types.Time(timeNow),
	}, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	var nextCursor types.StrInt64
	for {
		if err := queryOpenOrdersRateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("open order rate limiter wait error: %w", err)
		}

		req := e.v2client.NewGetUnfilledOrdersRequest().
			Symbol(symbol).
			Limit(strconv.FormatInt(queryLimit, 10))
		if nextCursor != 0 {
			req.IdLessThan(strconv.FormatInt(int64(nextCursor), 10))
		}

		openOrders, err := req.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query open orders: %w", err)
		}

		for _, o := range openOrders {
			order, err := unfilledOrderToGlobalOrder(o)
			if err != nil {
				return nil, fmt.Errorf("failed to convert order, err: %v", err)
			}

			orders = append(orders, *order)
		}

		orderLen := len(openOrders)
		// a defensive programming to ensure the length of order response is expected.
		if orderLen > queryLimit {
			return nil, fmt.Errorf("unexpected open orders length %d", orderLen)
		}

		if orderLen < queryLimit {
			break
		}
		nextCursor = openOrders[orderLen-1].OrderId
	}

	return orders, nil
}

// QueryClosedOrders queries closed order by time range(`CreatedTime`) and id. The order of the response is in descending order.
// If you need to retrieve all data, please utilize the function pkg/exchange/batch.ClosedOrderBatchQuery.
//
// REMARK: If your start time is 90 days earlier, we will update it to now - 90 days.
// ** Since is inclusive, Until is exclusive. If you use a time range to query, you must provide both a start time and an end time. **
// ** Since and Until cannot exceed 90 days. **
// ** Since from the last 90 days can be queried **
func (e *Exchange) QueryClosedOrders(
	ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64,
) (orders []types.Order, err error) {
	newSince := since
	now := e.timeNowFn()

	if time.Since(newSince) > maxHistoricalDataQueryPeriod {
		newSince = now.Add(-maxHistoricalDataQueryPeriod)
		log.Warnf("!!!BITGET EXCHANGE API NOTICE!!! The closed order API cannot query data beyond 90 days from the current date, update %s -> %s", since, newSince)
	}
	if until.Before(newSince) || until.Equal(newSince) {
		log.Warnf("!!!BITGET EXCHANGE API NOTICE!!! The 'until' comes before 'since', update until to now(%s -> %s).", until, now)
		until = now
	}
	if until.Sub(newSince) > maxHistoricalDataQueryPeriod {
		return nil, fmt.Errorf("the start time %s and end time %s cannot exceed 90 days", newSince, until)
	}
	if lastOrderID != 0 {
		log.Warn("!!!BITGET EXCHANGE API NOTICE!!! The order of response is in descending order, so the last order id not supported.")
	}

	if err := closedQueryOrdersRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query closed order rate limiter wait error: %w", err)
	}
	res, err := e.v2client.NewGetHistoryOrdersRequest().
		Symbol(symbol).
		Limit(strconv.Itoa(queryLimit)).
		StartTime(newSince).
		EndTime(until).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call get order histories error: %w", err)
	}

	for _, order := range res {
		o, err2 := toGlobalOrder(order)
		if err2 != nil {
			err = multierr.Append(err, err2)
			continue
		}

		if o.Status.Closed() {
			orders = append(orders, *o)
		}
	}
	if err != nil {
		return nil, err
	}

	return types.SortOrdersAscending(orders), nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (errs error) {
	if len(orders) == 0 {
		return nil
	}

	for _, order := range orders {
		req := e.v2client.NewCancelOrderRequest()

		reqId := ""
		switch {
		// use the OrderID first, then the ClientOrderID
		case order.OrderID > 0:
			req.OrderId(strconv.FormatUint(order.OrderID, 10))
			reqId = strconv.FormatUint(order.OrderID, 10)

		case len(order.ClientOrderID) != 0:
			req.ClientOrderId(order.ClientOrderID)
			reqId = order.ClientOrderID

		default:
			errs = multierr.Append(
				errs,
				fmt.Errorf("the order uuid and client order id are empty, order: %#v", order),
			)
			continue
		}

		req.Symbol(order.Symbol)

		if err := cancelOrderRateLimiter.Wait(ctx); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("cancel order rate limiter wait, orderId: %d, clientOrderId: %s, error: %w", order.OrderID, order.ClientOrderID, err))
			continue
		}
		res, err := req.Do(ctx)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to cancel orderId: %d, clientOrderId: %s, err: %w", order.OrderID, order.ClientOrderID, err))
			continue
		}

		// sanity check
		if res.OrderId.String() != reqId && res.ClientOrderId != reqId {
			errs = multierr.Append(errs, fmt.Errorf("order id mismatch, exp: %s, respOrderId: %d, respClientOrderId: %s", reqId, res.OrderId, res.ClientOrderId))
			continue
		}
	}

	return errs
}

// QueryTrades queries fill trades. The trade of the response is in descending order. The time-based query are typically
// using (`CreatedTime`) as the search criteria.
// If you need to retrieve all data, please utilize the function pkg/exchange/batch.TradeBatchQuery.
//
// REMARK: If your start time is 90 days earlier, we will update it to now - 90 days.
// ** StartTime is inclusive, EndTime is exclusive. If you use the EndTime, the StartTime is required. **
// ** StartTime and EndTime cannot exceed 90 days. **
func (e *Exchange) QueryTrades(
	ctx context.Context, symbol string, options *types.TradeQueryOptions,
) (trades []types.Trade, err error) {
	if options.LastTradeID != 0 {
		log.Warn("!!!BITGET EXCHANGE API NOTICE!!! The trade of response is in descending order, so the last trade id not supported.")
	}

	req := e.v2client.NewGetTradeFillsRequest()
	req.Symbol(symbol)

	var newStartTime time.Time
	if options.StartTime != nil {
		newStartTime = *options.StartTime
		if time.Since(newStartTime) > maxHistoricalDataQueryPeriod {
			newStartTime = e.timeNowFn().Add(-maxHistoricalDataQueryPeriod)
			log.Warnf("!!!BITGET EXCHANGE API NOTICE!!! The trade API cannot query data beyond 90 days from the current date, update %s -> %s", *options.StartTime, newStartTime)
		}
		req.StartTime(newStartTime)
	}

	if options.EndTime != nil {
		if newStartTime.IsZero() {
			return nil, errors.New("start time is required for query trades if you take end time")
		}
		if options.EndTime.Before(newStartTime) {
			return nil, fmt.Errorf("end time %s before start %s", *options.EndTime, newStartTime)
		}
		if options.EndTime.Sub(newStartTime) > maxHistoricalDataQueryPeriod {
			return nil, fmt.Errorf("start time %s and end time %s cannot greater than 90 days", newStartTime, options.EndTime)
		}
		req.EndTime(*options.EndTime)
	}

	limit := options.Limit
	if limit > queryLimit || limit <= 0 {
		log.Debugf("limtit is exceeded or zero, update to %d, got: %d", queryLimit, options.Limit)
		limit = queryLimit
	}
	req.Limit(strconv.FormatInt(limit, 10))

	if err := queryTradeRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("trade rate limiter wait error: %w", err)
	}
	response, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades, err: %w", err)
	}

	var errs error
	for _, trade := range response {
		res, err := toGlobalTrade(trade)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		trades = append(trades, *res)
	}

	if errs != nil {
		return nil, errs
	}

	return trades, nil
}
