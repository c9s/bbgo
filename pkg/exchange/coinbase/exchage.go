package coinbase

import (
	"context"
	"fmt"
	"strings"
	"time"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var (
	// https://docs.cdp.coinbase.com/exchange/docs/rate-limits
	//
	// Rate Limit: 10 requests per 2 seconds, Rate limit rule: UserID
	queryAccountLimiter = rate.NewLimiter(rate.Every(200*time.Millisecond), 1)
	// Rate Limit: 10 requests per second
	queryMarketDataLimiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 1)

	// compile time check: implemented interface
	_ types.Exchange                  = &Exchange{}
	_ types.ExchangeMarketDataService = &Exchange{}
	_ types.CustomIntervalProvider    = &Exchange{}
)

const (
	ID                = "coinbase"
	PlatformToken     = "COIN"
	PaginationLimit   = 100
	DefaultKLineLimit = 300
)

var log = logrus.WithField("exchange", ID)

type Exchange struct {
	client *api.RestAPIClient

	// api keys
	apiKey        string
	apiSecret     string
	apiPassphrase string
}

func New(key, secret, passphrase string, timeout time.Duration) *Exchange {
	client := api.NewClient(key, secret, passphrase, timeout)
	return &Exchange{
		client: client,

		apiKey:        key,
		apiSecret:     secret,
		apiPassphrase: passphrase,
	}
}

// CustomIntervalProvider
var supportedIntervalMap = map[types.Interval]int{
	types.Interval1m:  60,
	types.Interval5m:  5 * 60,
	types.Interval15m: 15 * 60,
	types.Interval1h:  60 * 60,
	types.Interval6h:  6 * 60 * 60,
	types.Interval1d:  24 * 60 * 60,
}

func (e *Exchange) SupportedInterval() map[types.Interval]int {
	return supportedIntervalMap
}

func (e *Exchange) IsSupportedInterval(interval types.Interval) bool {
	_, ok := supportedIntervalMap[interval]
	return ok
}

// ExchangeMinimal
func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeCoinBase
}

func (e *Exchange) PlatformFeeCurrency() string {
	return PlatformToken
}

// ExchangeAccountService
func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	if err := queryAccountLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	balances, err := e.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}
	account := types.NewAccount()
	account.UpdateBalances(balances)
	return account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	if err := queryAccountLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	req := e.client.NewGetBalancesRequest()
	accounts, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}
	balances := make(types.BalanceMap)
	for _, cbBalance := range accounts {
		cur := strings.ToUpper(cbBalance.Currency)
		tb := balances[cur]
		balances[cur] = tb.Add(toGlobalBalance(cur, &cbBalance))
	}
	return balances, nil
}

// ExchangeTradeService
func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	if len(order.Market.Symbol) == 0 {
		return nil, fmt.Errorf("order.Market.Symbol is required: %+v", order)
	}
	req := e.client.NewCreateOrderRequest().ProductID(toLocalSymbol(order.Market.Symbol))

	// set order type
	switch order.Type {
	case types.OrderTypeLimit:
		req.OrderType("limit")
	case types.OrderTypeMarket:
		req.OrderType("market")
	default:
		return nil, fmt.Errorf("unsupported order type: %v", order.Type)
	}
	// set side
	switch order.Side {
	case types.SideTypeBuy:
		req.Side("buy")
	case types.SideTypeSell:
		req.Side("sell")
	default:
		return nil, fmt.Errorf("unsupported order side: %v", order.Side)
	}
	// set quantity
	qty := order.Quantity
	if order.Type == types.OrderTypeMarket && order.Side == types.SideTypeBuy {
		ticker, err := e.QueryTicker(ctx, order.Market.Symbol)
		if err != nil {
			return nil, err
		}
		qty = qty.Mul(ticker.Buy)
	}
	req.Size(qty.String())
	// set price
	if order.Type == types.OrderTypeLimit {
		req.Price(order.Price)
	}

	// set time in force, using default if not set
	if len(order.TimeInForce) > 0 {
		switch order.TimeInForce {
		case types.TimeInForceGTC:
			req.TimeInForce("GTC")
		case types.TimeInForceIOC:
			req.TimeInForce("IOC")
		case types.TimeInForceFOK:
			req.TimeInForce("FOK")
		case types.TimeInForceGTT:
			req.TimeInForce("GTT")
		default:
			return nil, fmt.Errorf("unsupported time in force: %v", order.TimeInForce)
		}
	}
	// client order id
	if len(order.ClientOrderID) > 0 {
		req.ClientOrderID(order.ClientOrderID)
	}

	timeNow := time.Now()
	res, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}
	return &types.Order{
		SubmitOrder:      order,
		Exchange:         types.ExchangeCoinBase,
		UUID:             res.ID,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: fixedpoint.Zero,
		IsWorking:        true,
		CreationTime:     types.Time(timeNow),
		UpdateTime:       types.Time(timeNow),
	}, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error) {
	cbOrders, err := e.queryOrdersByPagination(ctx, toLocalSymbol(symbol), []string{"open"})
	if err != nil {
		return nil, err
	}
	orders := make([]types.Order, 0, len(cbOrders))
	for _, cbOrder := range cbOrders {
		orders = append(orders, toGlobalOrder(&cbOrder))
	}
	return orders, nil
}

func (e *Exchange) queryOrdersByPagination(ctx context.Context, symbol string, status []string) ([]api.Order, error) {
	if err := queryAccountLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	sortedBy := "created_at"
	sorting := "desc"
	localSymbol := toLocalSymbol(symbol)
	getOrdersReq := e.client.NewGetOrdersRequest()
	getOrdersReq.ProductID(localSymbol).Status(status).SortedBy(sortedBy).Sorting(sorting).Limit(PaginationLimit)
	cbOrders, err := getOrdersReq.Do(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get orders")
	}

	if len(cbOrders) < PaginationLimit {
		return cbOrders, nil
	}

	done := false
	for {
		select {
		case <-ctx.Done():
			return cbOrders, ctx.Err()
		default:
			after := time.Time(cbOrders[len(cbOrders)-1].CreatedAt)
			getOrdersReq.After(after)
			newOrders, err := getOrdersReq.Do(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get orders while paginating")
			}
			if len(newOrders) < PaginationLimit {
				done = true
			}
			cbOrders = append(cbOrders, newOrders...)
		}
		if done {
			break
		}
	}
	return cbOrders, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	var failedOrderIDs []string
	for _, order := range orders {
		req := e.client.NewCancelOrderRequest().OrderID(order.UUID)
		res, err := req.Do(ctx)
		if err != nil {
			log.WithError(err).Errorf("failed to cancel order: %v", order.UUID)
			failedOrderIDs = append(failedOrderIDs, order.UUID)
		} else {
			log.Infof("order %v has been cancelled", res)
		}
	}
	if len(failedOrderIDs) > 0 {
		return errors.Errorf("failed to cancel orders: %v", failedOrderIDs)
	}
	return nil
}

// ExchangeMarketDataService
func (e *Exchange) NewStream() types.Stream {
	return NewStream(e, e.apiKey, e.apiPassphrase, e.apiSecret)
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	reqMarketInfo := e.client.NewGetMarketInfoRequest()
	markets, err := reqMarketInfo.Do(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get markets")
	}

	marketMap := make(types.MarketMap)
	for _, m := range markets {
		// skip products that is not online
		if m.Status != "online" {
			continue
		}
		marketMap[toGlobalSymbol(m.ID)] = toGlobalMarket(&m)
	}
	return marketMap, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	req := e.client.NewGetTickerRequest().ProductID(toLocalSymbol(symbol))
	cbTicker, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get ticker: %v", symbol)
	}
	ticker := toGlobalTicker(cbTicker)
	return &ticker, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	tickers := make(map[string]types.Ticker)
	for _, s := range symbol {
		ticker, err := e.QueryTicker(ctx, s)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get ticker for %v", s)
		}
		tickers[s] = *ticker
	}
	return tickers, nil
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	if !e.IsSupportedInterval(interval) {
		return nil, errors.Errorf("unsupported interval: %v", interval)
	}
	// default limit is 300, which is the maximum limit of the Coinbase Exchange API
	if options.Limit == 0 {
		options.Limit = DefaultKLineLimit
	}
	if options.Limit > DefaultKLineLimit {
		log.Warnf("limit %d is greater than the maximum limit 300, set to 300", options.Limit)
		options.Limit = DefaultKLineLimit
	}
	granularity := fmt.Sprintf("%d", interval.Seconds())
	req := e.client.NewGetCandlesRequest().ProductID(toLocalSymbol(symbol)).Granularity(granularity)
	if options.StartTime != nil {
		req.Start(*options.StartTime)
	}
	if options.EndTime != nil {
		req.End(*options.EndTime)
	}
	rawCandles, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get klines(%v): %v", interval, symbol)
	}
	candles := make([]api.Candle, 0, len(rawCandles))
	for _, rawCandle := range rawCandles {
		candle, err := rawCandle.Candle()
		if err != nil {
			log.Warnf("invalid raw candle detected, skipping: %v", rawCandle)
			continue
		}
		candles = append(candles, *candle)
	}

	klines := make([]types.KLine, 0, len(candles))
	for _, candle := range candles {
		kline := toGlobalKline(symbol, interval, &candle)
		klines = append(klines, kline)
	}
	return klines, nil
}

// ExchangeOrderQueryService
func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	req := e.client.NewSingleOrderRequst().OrderID(q.OrderID)
	cbOrder, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get order: %v", q.OrderID)
	}
	order := toGlobalOrder(cbOrder)
	return &order, nil
}

func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
	cbTrades, err := e.queryOrderTradesByPagination(ctx, q)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get order trades: %v", q.OrderID)
	}
	trades := make([]types.Trade, 0, len(cbTrades))
	for _, cbTrade := range cbTrades {
		trades = append(trades, toGlobalTrade(&cbTrade))
	}
	return trades, nil
}

func (e *Exchange) queryOrderTradesByPagination(ctx context.Context, q types.OrderQuery) (api.TradeSnapshot, error) {
	req := e.client.NewGetOrderTradesRequest().Limit(PaginationLimit)
	if len(q.OrderID) > 0 {
		req.OrderID(q.OrderID)
	}
	if len(q.Symbol) > 0 {
		req.ProductID(toLocalSymbol(q.Symbol))
	}
	cbTrades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(cbTrades) < PaginationLimit {
		return cbTrades, nil
	}
	done := false
	for {
		select {
		case <-ctx.Done():
			return cbTrades, ctx.Err()
		default:
			lastTrade := cbTrades[len(cbTrades)-1]
			req.After(lastTrade.TradeID)
			newTrades, err := req.Do(ctx)
			if err != nil {
				return nil, err
			}
			if len(newTrades) < PaginationLimit {
				done = true
			}
			cbTrades = append(cbTrades, newTrades...)
		}
		if done {
			break
		}
	}
	return cbTrades, nil
}
