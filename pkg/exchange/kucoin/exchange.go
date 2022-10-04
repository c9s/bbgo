package kucoin

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var marketDataLimiter = rate.NewLimiter(rate.Every(6*time.Second), 1)
var queryTradeLimiter = rate.NewLimiter(rate.Every(6*time.Second), 1)
var queryOrderLimiter = rate.NewLimiter(rate.Every(6*time.Second), 1)

var ErrMissingSequence = errors.New("sequence is missing")

// OKB is the platform currency of OKEx, pre-allocate static string here
const KCS = "KCS"

var log = logrus.WithFields(logrus.Fields{
	"exchange": "kucoin",
})

type Exchange struct {
	key, secret, passphrase string
	client                  *kucoinapi.RestClient
}

func New(key, secret, passphrase string) *Exchange {
	client := kucoinapi.NewClient()

	// for public access mode
	if len(key) > 0 && len(secret) > 0 && len(passphrase) > 0 {
		client.Auth(key, secret, passphrase)
	}

	return &Exchange{
		key: key,
		// pragma: allowlist nextline secret
		secret:     secret,
		passphrase: passphrase,
		client:     client,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeKucoin
}

func (e *Exchange) PlatformFeeCurrency() string {
	return KCS
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	req := e.client.AccountService.NewListAccountsRequest()
	accounts, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	// for now, we only return the trading account
	a := types.NewAccount()
	balances := toGlobalBalanceMap(accounts)
	a.UpdateBalances(balances)
	return a, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	req := e.client.AccountService.NewListAccountsRequest()
	accounts, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalBalanceMap(accounts), nil
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	markets, err := e.client.MarketDataService.ListSymbols()
	if err != nil {
		return nil, err
	}

	marketMap := types.MarketMap{}
	for _, s := range markets {
		market := toGlobalMarket(s)
		marketMap.Add(market)
	}

	return marketMap, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	s, err := e.client.MarketDataService.GetTicker24HStat(symbol)
	if err != nil {
		return nil, err
	}

	ticker := toGlobalTicker(*s)
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

	allTickers, err := e.client.MarketDataService.ListTickers()
	if err != nil {
		return nil, err
	}

	for _, s := range allTickers.Ticker {
		tickers[s.Symbol] = toGlobalTicker(s)
	}

	return tickers, nil
}

// From the doc
// Type of candlestick patterns: 1min, 3min, 5min, 15min, 30min, 1hour, 2hour, 4hour, 6hour, 8hour, 12hour, 1day, 1week
var supportedIntervals = map[types.Interval]int{
	types.Interval1m:  1 * 60,
	types.Interval5m:  5 * 60,
	types.Interval15m: 15 * 60,
	types.Interval30m: 30 * 60,
	types.Interval1h:  60 * 60,
	types.Interval2h:  60 * 60 * 2,
	types.Interval4h:  60 * 60 * 4,
	types.Interval6h:  60 * 60 * 6,
	// types.Interval8h: 60 * 60 * 8,
	types.Interval12h: 60 * 60 * 12,
}

func (e *Exchange) SupportedInterval() map[types.Interval]int {
	return supportedIntervals
}

func (e *Exchange) IsSupportedInterval(interval types.Interval) bool {
	_, ok := supportedIntervals[interval]
	return ok
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	if err := marketDataLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	req := e.client.MarketDataService.NewGetKLinesRequest()
	req.Symbol(toLocalSymbol(symbol))
	req.Interval(toLocalInterval(interval))
	if options.StartTime != nil {
		req.StartAt(*options.StartTime)
		// For each query, the system would return at most **1500** pieces of data. To obtain more data, please page the data by time.
		req.EndAt(options.StartTime.Add(1500 * interval.Duration()))
	} else if options.EndTime != nil {
		req.EndAt(*options.EndTime)
	}

	ks, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var klines []types.KLine
	for _, k := range ks {
		gi := toGlobalInterval(k.Interval)
		klines = append(klines, types.KLine{
			Exchange:    types.ExchangeKucoin,
			Symbol:      toGlobalSymbol(k.Symbol),
			StartTime:   types.Time(k.StartTime),
			EndTime:     types.Time(k.StartTime.Add(gi.Duration() - time.Millisecond)),
			Interval:    gi,
			Open:        k.Open,
			Close:       k.Close,
			High:        k.High,
			Low:         k.Low,
			Volume:      k.Volume,
			QuoteVolume: k.QuoteVolume,
			Closed:      true,
		})
	}

	sort.Slice(klines, func(i, j int) bool {
		return klines[i].StartTime.Before(klines[j].StartTime.Time())
	})

	return klines, nil
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	req := e.client.TradeService.NewPlaceOrderRequest()
	req.Symbol(toLocalSymbol(order.Symbol))
	req.Side(toLocalSide(order.Side))

	if order.ClientOrderID != "" {
		req.ClientOrderID(order.ClientOrderID)
	}

	if order.Market.Symbol != "" {
		req.Size(order.Market.FormatQuantity(order.Quantity))
	} else {
		// TODO: report error?
		req.Size(order.Quantity.FormatString(8))
	}

	// set price field for limit orders
	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		if order.Market.Symbol != "" {
			req.Price(order.Market.FormatPrice(order.Price))
		} else {
			// TODO: report error?
			req.Price(order.Price.FormatString(8))
		}
	}

	if order.Type == types.OrderTypeLimitMaker {
		req.PostOnly(true)
	}

	switch order.TimeInForce {
	case "FOK":
		req.TimeInForce(kucoinapi.TimeInForceFOK)
	case "IOC":
		req.TimeInForce(kucoinapi.TimeInForceIOC)
	default:
		// default to GTC
		req.TimeInForce(kucoinapi.TimeInForceGTC)
	}

	switch order.Type {
	case types.OrderTypeStopLimit:
		req.OrderType(kucoinapi.OrderTypeStopLimit)

	case types.OrderTypeLimit, types.OrderTypeLimitMaker:
		req.OrderType(kucoinapi.OrderTypeLimit)

	case types.OrderTypeMarket:
		req.OrderType(kucoinapi.OrderTypeMarket)
	}

	orderResponse, err := req.Do(ctx)
	if err != nil {
		return createdOrder, err
	}

	return &types.Order{
		SubmitOrder:      order,
		Exchange:         types.ExchangeKucoin,
		OrderID:          hashStringID(orderResponse.OrderID),
		UUID:             orderResponse.OrderID,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: fixedpoint.Zero,
		IsWorking:        true,
		CreationTime:     types.Time(time.Now()),
		UpdateTime:       types.Time(time.Now()),
	}, nil
}

// QueryOpenOrders
/*
Documentation from the Kucoin API page

Any order on the exchange order book is in active status.
Orders removed from the order book will be marked with done status.
After an order becomes done, there may be a few milliseconds latency before it’s fully settled.

You can check the orders in any status.
If the status parameter is not specified, orders of done status will be returned by default.

When you query orders in active status, there is no time limit.
However, when you query orders in done status, the start and end time range cannot exceed 7* 24 hours.
An error will occur if the specified time window exceeds the range.

If you specify the end time only, the system will automatically calculate the start time as end time minus 7*24 hours, and vice versa.

The history for cancelled orders is only kept for one month.
You will not be able to query for cancelled orders that have happened more than a month ago.
*/
func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	req := e.client.TradeService.NewListOrdersRequest()
	req.Symbol(toLocalSymbol(symbol))
	req.Status("active")
	orderList, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: support pagination (right now we can only get 50 items from the first page)
	for _, o := range orderList.Items {
		order := toGlobalOrder(o)
		orders = append(orders, order)
	}

	return orders, err
}

func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	req := e.client.TradeService.NewListOrdersRequest()
	req.Symbol(toLocalSymbol(symbol))
	req.Status("done")
	req.StartAt(since)

	// kucoin:
	// When you query orders in active status, there is no time limit.
	// However, when you query orders in done status, the start and end time range cannot exceed 7* 24 hours.
	// An error will occur if the specified time window exceeds the range.
	// If you specify the end time only, the system will automatically calculate the start time as end time minus 7*24 hours, and vice versa.
	if until.Sub(since) < 7*24*time.Hour {
		req.EndAt(until)
	}

	if err := queryOrderLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	orderList, err := req.Do(ctx)
	if err != nil {
		return orders, err
	}

	for _, o := range orderList.Items {
		order := toGlobalOrder(o)
		orders = append(orders, order)
	}

	return orders, err
}

var launchDate = time.Date(2017, 9, 0, 0, 0, 0, 0, time.UTC)

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	req := e.client.TradeService.NewGetFillsRequest()
	req.Symbol(toLocalSymbol(symbol))

	// we always sync trades in the ascending order, and kucoin does not support last trade ID query
	// hence we need to set the start time here
	if options.StartTime != nil && options.StartTime.Before(launchDate) {
		// copy the time data object
		t := launchDate
		options.StartTime = &t
	}

	if options.StartTime != nil && options.EndTime != nil {
		req.StartAt(*options.StartTime)

		if options.EndTime.Sub(*options.StartTime) < 7*24*time.Hour {
			req.EndAt(*options.EndTime)
		}
	} else if options.StartTime != nil {
		req.StartAt(*options.StartTime)
	} else if options.EndTime != nil {
		req.EndAt(*options.EndTime)
	}

	if err := queryTradeLimiter.Wait(ctx); err != nil {
		return trades, err
	}

	response, err := req.Do(ctx)
	if err != nil {
		return trades, err
	}

	for _, fill := range response.Items {
		trade := toGlobalTrade(fill)
		trades = append(trades, trade)
	}

	return trades, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (errs error) {
	for _, o := range orders {
		req := e.client.TradeService.NewCancelOrderRequest()

		if o.UUID != "" {
			req.OrderID(o.UUID)
		} else if o.ClientOrderID != "" {
			req.ClientOrderID(o.ClientOrderID)
		} else {
			errs = multierr.Append(
				errs,
				fmt.Errorf("the order uuid or client order id is empty, order: %#v", o),
			)
			continue
		}

		response, err := req.Do(ctx)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		log.Infof("cancelled orders: %v", response.CancelledOrderIDs)
	}

	return errors.Wrap(errs, "order cancel error")
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.client, e)
}

func (e *Exchange) QueryDepth(ctx context.Context, symbol string) (types.SliceOrderBook, int64, error) {
	orderBook, err := e.client.MarketDataService.GetOrderBook(toLocalSymbol(symbol), 100)
	if err != nil {
		return types.SliceOrderBook{}, 0, err
	}

	if len(orderBook.Sequence) == 0 {
		return types.SliceOrderBook{}, 0, ErrMissingSequence
	}

	sequence, err := strconv.ParseInt(orderBook.Sequence, 10, 64)
	if err != nil {
		return types.SliceOrderBook{}, 0, err
	}

	return types.SliceOrderBook{
		Symbol: toGlobalSymbol(symbol),
		Bids:   orderBook.Bids,
		Asks:   orderBook.Asks,
	}, sequence, nil
}
