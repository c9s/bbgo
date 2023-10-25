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
	v3 "github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi/v3"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	maxOrderIdLen     = 36
	defaultQueryLimit = 50
	defaultKLineLimit = 1000

	halfYearDuration = 6 * 30 * 24 * time.Hour
)

// https://bybit-exchange.github.io/docs/zh-TW/v5/rate-limit
// GET/POST method (shared): 120 requests per second for 5 consecutive seconds
var (
	// sharedRateLimiter indicates that the API belongs to the public API.
	// The default order limiter apply 5 requests per second and a 5 initial bucket
	// this includes QueryMarkets, QueryTicker, QueryAccountBalances, GetFeeRates
	sharedRateLimiter          = rate.NewLimiter(rate.Every(time.Second/5), 5)
	queryOrderTradeRateLimiter = rate.NewLimiter(rate.Every(time.Second/5), 5)
	orderRateLimiter           = rate.NewLimiter(rate.Every(time.Second/10), 10)
	closedOrderQueryLimiter    = rate.NewLimiter(rate.Every(time.Second), 1)

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
	v3client    *v3.Client
}

func New(key, secret string) (*Exchange, error) {
	client, err := bybitapi.NewClient()
	if err != nil {
		return nil, err
	}

	if len(key) > 0 && len(secret) > 0 {
		client.Auth(key, secret)
	}

	return &Exchange{
		key: key,
		// pragma: allowlist nextline secret
		secret:   secret,
		client:   client,
		v3client: v3.NewClient(client),
	}, nil
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
		return nil, fmt.Errorf("unexpected ticker lenght, exp:1, got:%d", len(s.List))
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

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	cursor := ""
	for {
		req := e.client.NewGetOpenOrderRequest().Symbol(symbol)
		if len(cursor) != 0 {
			// the default limit is 20.
			req = req.Cursor(cursor)
		}

		if err = queryOrderTradeRateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("place order rate limiter wait error: %w", err)
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
	if len(res.List) != 1 {
		return nil, fmt.Errorf("unexpected order length, queryConfig: %+v", q)
	}

	return toGlobalOrder(res.List[0])
}

func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) (trades []types.Trade, err error) {
	if len(q.ClientOrderID) != 0 {
		log.Warn("!!!BYBIT EXCHANGE API NOTICE!!! Bybit does not support searching for trades using OrderClientId.")
	}

	if len(q.OrderID) == 0 {
		return nil, errors.New("orderID is required parameter")
	}
	req := e.v3client.NewGetTradesRequest().OrderId(q.OrderID)

	if len(q.Symbol) != 0 {
		req.Symbol(q.Symbol)
	}

	if err := queryOrderTradeRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("trade rate limiter wait error: %w", err)
	}
	response, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query order trades, err: %w", err)
	}

	var errs error
	for _, trade := range response.List {
		res, err := v3ToGlobalTrade(trade)
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

	// set quantity
	orderQty := order.Quantity
	// if the order is market buy, the quantity is quote coin, instead of base coin. so we need to convert it.
	if order.Type == types.OrderTypeMarket && order.Side == types.SideTypeBuy {
		ticker, err := e.QueryTicker(ctx, order.Market.Symbol)
		if err != nil {
			return nil, err
		}
		orderQty = order.Quantity.Mul(ticker.Buy)
	}
	req.Qty(order.Market.FormatQuantity(orderQty))

	// set price
	switch order.Type {
	case types.OrderTypeLimit:
		req.Price(order.Market.FormatPrice(order.Price))
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

	if err := orderRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("place order rate limiter wait error: %w", err)
	}
	res, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to place order, order: %#v, err: %w", order, err)
	}

	if len(res.OrderId) == 0 || (len(order.ClientOrderID) != 0 && res.OrderLinkId != order.ClientOrderID) {
		return nil, fmt.Errorf("unexpected order id, resp: %#v, order: %#v", res, order)
	}

	ordersResp, err := e.client.NewGetOpenOrderRequest().OrderId(res.OrderId).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query order by client order id: %s, err: %w", res.OrderLinkId, err)
	}

	if len(ordersResp.List) != 1 {
		return nil, fmt.Errorf("unexpected order length, client order id: %s", res.OrderLinkId)
	}

	return toGlobalOrder(ordersResp.List[0])
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

		if err := orderRateLimiter.Wait(ctx); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("cancel order rate limiter wait, order id: %s, error: %w", order.ClientOrderID, err))
			continue
		}
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

func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, util time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	if !since.IsZero() || !util.IsZero() {
		log.Warn("!!!BYBIT EXCHANGE API NOTICE!!! the since/until conditions will not be effected on SPOT account, bybit exchange does not support time-range-based query currently")
	}

	if err := closedOrderQueryLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query closed order rate limiter wait error: %w", err)
	}
	res, err := e.client.NewGetOrderHistoriesRequest().
		Symbol(symbol).
		Cursor(strconv.FormatUint(lastOrderID, 10)).
		Limit(defaultQueryLimit).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call get order histories error: %w", err)
	}

	for _, order := range res.List {
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

/*
QueryTrades queries trades by time range or trade id range.
If options.StartTime is not specified, you can only query for records in the last 7 days.
If you want to query for records older than 7 days, options.StartTime is required.
It supports to query records up to 180 days.

** Here includes MakerRebate. If needed, let's discuss how to modify it to return in trade. **
** StartTime and EndTime are inclusive. **
** StartTime and EndTime cannot exceed 180 days. **
** StartTime, EndTime, FromTradeId can be used together. **
** If the `FromTradeId` is passed, and `ToTradeId` is null, then the result is sorted by tradeId in `ascend`.
Otherwise, the result is sorted by tradeId in `descend`. **
*/
func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	// using v3 client, since the v5 API does not support feeCurrency.
	req := e.v3client.NewGetTradesRequest()
	req.Symbol(symbol)

	// If `lastTradeId` is given and greater than 0, the query will use it as a condition and the retrieved result will be
	// in `ascending` order. We can use `lastTradeId` to retrieve all the data. So we hack it to '1' if `lastTradeID` is '0'.
	// If 0 is given, it will not be used as a condition and the result will be in `descending` order. The FromTradeId
	// option cannot be used to retrieve more data.
	req.FromTradeId(strconv.FormatUint(options.LastTradeID, 10))
	if options.LastTradeID == 0 {
		req.FromTradeId("1")
	}
	if options.StartTime != nil {
		req.StartTime(options.StartTime.UTC())
	}
	if options.EndTime != nil {
		req.EndTime(options.EndTime.UTC())
	}

	limit := uint64(options.Limit)
	if limit > defaultQueryLimit || limit <= 0 {
		log.Debugf("limtit is exceeded or zero, update to %d, got: %d", defaultQueryLimit, options.Limit)
		limit = defaultQueryLimit
	}
	req.Limit(limit)

	if err := queryOrderTradeRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("trade rate limiter wait error: %w", err)
	}
	response, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades, err: %w", err)
	}

	var errs error
	for _, trade := range response.List {
		res, err := v3ToGlobalTrade(trade)
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
	if err := sharedRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query account balances rate limiter wait error: %w", err)
	}

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
func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	req := e.client.NewGetKLinesRequest().Symbol(symbol)
	intervalStr, err := toLocalInterval(interval)
	if err != nil {
		return nil, err
	}
	req.Interval(intervalStr)

	limit := uint64(options.Limit)
	if limit > defaultKLineLimit || limit <= 0 {
		log.Debugf("limtit is exceeded or zero, update to %d, got: %d", defaultKLineLimit, options.Limit)
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
