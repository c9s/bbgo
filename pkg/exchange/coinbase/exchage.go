package coinbase

import (
	"context"
	"strings"
	"time"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var (
	// Rate Limit: 10 requests per 2 seconds, Rate limit rule: UserID
	queryAccountLimiter = rate.NewLimiter(rate.Every(200*time.Millisecond), 1)

	// compile time check: implemented interface
	_ types.Exchange                  = &Exchange{}
	_ types.ExchangeMarketDataService = &Exchange{}
	_ types.CustomIntervalProvider    = &Exchange{}
)

const (
	ID            = "coinbase"
	PlatformToken = "COIN"
)

var log = logrus.WithField("exchange", ID)

type Exchange struct {
	client *api.RestAPIClient
}

func New(key, secret, passphrase string, timeout time.Duration) *Exchange {
	client := api.NewClient(key, secret, passphrase, timeout)
	return &Exchange{
		client: &client,
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

	accounts, err := e.client.GetBalances(ctx)
	if err != nil {
		return nil, err
	}
	balances := make(types.BalanceMap)
	for _, b := range accounts {
		cur := strings.ToUpper(b.Currency)
		balances[cur] = types.Balance{
			Currency:  cur,
			Available: b.Available,
		}
	}
	return balances, nil
}

// ExchangeTradeService
func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	res, err := e.client.CreateOrder(ctx, order)
	if err != nil {
		return nil, err
	}
	return &types.Order{
		Exchange: types.ExchangeCoinBase,
		UUID:     res.ID,
	}, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error) {
	cbOrders, err := e.queryOrdersByPagination(ctx, toLocalSymbol(symbol), []string{"open"})
	if err != nil {
		return nil, err
	}
	orders := make([]types.Order, 0, len(cbOrders))
	for _, cbOrder := range cbOrders {
		orders = append(orders, *toGlobalOrder(&cbOrder))
	}
	return orders, nil
}

func (e *Exchange) queryOrdersByPagination(ctx context.Context, symbol string, status []string) ([]api.Order, error) {
	sortedBy := "created_at"
	sorting := "desc"
	before := time.Now()
	localSymbol := toLocalSymbol(symbol)
	paginationLimit := 1000
	cbOrders, err := e.client.GetOrders(ctx, localSymbol, status, paginationLimit, &sortedBy, &sorting, &before)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get orders")
	}

	done := false
	if len(cbOrders) < 1000 || len(cbOrders) == 0 {
		done = true
	}
	for {
		if done {
			break
		}

		before = cbOrders[len(cbOrders)-1].CreatedAt
		new_orders, err := e.client.GetOrders(ctx, localSymbol, status, paginationLimit, &sortedBy, &sorting, &before)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get orders while paginating")
		}
		if len(new_orders) < paginationLimit {
			done = true
		}
		cbOrders = append(cbOrders, new_orders...)
	}
	return cbOrders, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	failedOrderIDs := make([]string, 0)
	for _, order := range orders {
		req := e.client.NewCancelOrderRequest(order.UUID)
		res, err := req.Do(ctx)
		if err != nil {
			log.WithError(err).Errorf("failed to cancel order: %v", order.UUID)
			failedOrderIDs = append(failedOrderIDs, order.UUID)
		}
		log.Infof("order %v has been cancelled", res)
	}
	if len(failedOrderIDs) > 0 {
		return errors.Errorf("failed to cancel orders: %v", failedOrderIDs)
	}
	return nil
}

// ExchangeMarketDataService
func (e *Exchange) NewStream() types.Stream {
	return nil
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	markets, err := e.client.GetMarketInfo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get markets")
	}
	marketMap := make(types.MarketMap)
	for _, m := range markets {
		marketMap[toGlobalSymbol(m.ID)] = *toGlobalMarket(&m)
	}
	return marketMap, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	req := e.client.NewGetTickerRequest(toLocalSymbol(symbol))
	ticker, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get ticker: %v", symbol)
	}
	return toGlobalTicker(ticker), nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	tickers := make(map[string]types.Ticker)
	for _, s := range symbol {
		ticker, err := e.QueryTicker(ctx, s)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get tickers")
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
		options.Limit = 300
	}
	if options.Limit > 300 {
		log.Warnf("limit %d is greater than the maximum limit 300, set to 300", options.Limit)
		options.Limit = 300
	}
	var start, end *string
	granity := interval.String()
	req := e.client.NewGetCandlesRequest(toLocalSymbol(symbol), &granity, start, end)
	candles, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get klines(%v): %v", interval, symbol)
	}
	if len(candles) > options.Limit {
		candles = candles[:options.Limit]
	}
	klines := make([]types.KLine, 0, len(candles))
	for _, candle := range candles {
		klines = append(klines, *toGlobalKline(symbol, granity, &candle))
	}
	return klines, nil
}

// ExchangeOrderQueryService
func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	cbOrder, err := e.client.GetSingleOrder(ctx, q.OrderID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get order: %v", q.OrderID)
	}
	return toGlobalOrder(cbOrder), nil
}

func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
	cbTrades, err := e.queryOrderTradesByPagination(ctx, q.OrderID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get order trades: %v", q.OrderID)
	}
	trades := make([]types.Trade, 0, len(cbTrades))
	for _, cbTrade := range cbTrades {
		trades = append(trades, *toGlobalTrade(&cbTrade))
	}
	return trades, nil
}

func (e *Exchange) queryOrderTradesByPagination(ctx context.Context, orderID string) (api.TradeSnapshot, error) {
	paginationLimit := 100
	cbTrades, err := e.client.GetOrderTrades(ctx, orderID, paginationLimit, nil)
	if err != nil {
		return nil, err
	}
	if len(cbTrades) < paginationLimit {
		return cbTrades, nil
	}
	done := false
	if len(cbTrades) < paginationLimit || len(cbTrades) == 0 {
		done = true
	}
	for {
		if done {
			break
		}
		lastTrade := cbTrades[len(cbTrades)-1]
		newTrades, err := e.client.GetOrderTrades(ctx, orderID, paginationLimit, &lastTrade.OrderID)
		if err != nil {
			return nil, err
		}
		if len(newTrades) < paginationLimit {
			done = true
		}
		cbTrades = append(cbTrades, newTrades...)
	}
	return cbTrades, nil
}
