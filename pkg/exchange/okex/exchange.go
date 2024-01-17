package okex

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// Okex rate limit list in each api document
// The default order limiter apply 30 requests per second and a 5 initial bucket
// this includes QueryOrder, QueryOrderTrades, SubmitOrder, QueryOpenOrders, CancelOrders
// Market data limiter means public api, this includes QueryMarkets, QueryTicker, QueryTickers, QueryKLines
var (
	marketDataLimiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 5)

	queryMarketLimiter          = rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
	queryTickerLimiter          = rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
	queryTickersLimiter         = rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
	queryAccountLimiter         = rate.NewLimiter(rate.Every(200*time.Millisecond), 5)
	placeOrderLimiter           = rate.NewLimiter(rate.Every(30*time.Millisecond), 30)
	batchCancelOrderLimiter     = rate.NewLimiter(rate.Every(5*time.Millisecond), 200)
	queryOpenOrderLimiter       = rate.NewLimiter(rate.Every(30*time.Millisecond), 30)
	queryClosedOrderRateLimiter = rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
	queryTradeLimiter           = rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
)

const (
	ID = "okex"

	// PlatformToken is the platform currency of OKEx, pre-allocate static string here
	PlatformToken = "OKB"

	defaultQueryLimit = 100

	maxHistoricalDataQueryPeriod = 90 * 24 * time.Hour
)

var log = logrus.WithFields(logrus.Fields{
	"exchange": ID,
})

var ErrSymbolRequired = errors.New("symbol is a required parameter")

type Exchange struct {
	key, secret, passphrase string

	client *okexapi.RestClient
}

func New(key, secret, passphrase string) *Exchange {
	client := okexapi.NewClient()

	if len(key) > 0 && len(secret) > 0 {
		client.Auth(key, secret, passphrase)
	}

	return &Exchange{
		key:        key,
		secret:     secret,
		passphrase: passphrase,
		client:     client,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeOKEx
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	if err := queryMarketLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("markets rate limiter wait error: %w", err)
	}

	instruments, err := e.client.NewGetInstrumentsInfoRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, instrument := range instruments {
		symbol := toGlobalSymbol(instrument.InstrumentID)
		market := types.Market{
			Symbol:      symbol,
			LocalSymbol: instrument.InstrumentID,

			QuoteCurrency: instrument.QuoteCurrency,
			BaseCurrency:  instrument.BaseCurrency,

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
		}
		markets[symbol] = market
	}

	return markets, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	if err := queryTickerLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("ticker rate limiter wait error: %w", err)
	}

	symbol = toLocalSymbol(symbol)
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

	marketTickers, err := e.client.NewGetTickersRequest().Do(ctx)
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
	bals, err := e.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}

	account := types.NewAccount()
	account.UpdateBalances(bals)
	return account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	if err := queryAccountLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("account rate limiter wait error: %w", err)
	}

	accountBalances, err := e.client.NewGetAccountInfoRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(accountBalances) != 1 {
		return nil, fmt.Errorf("unexpected length of balances: %v", accountBalances)
	}

	return toGlobalBalance(&accountBalances[0]), nil
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	orderReq := e.client.NewPlaceOrderRequest()

	orderReq.InstrumentID(toLocalSymbol(order.Symbol))
	orderReq.Side(toLocalSideType(order.Side))
	orderReq.Size(order.Market.FormatQuantity(order.Quantity))

	// set price field for limit orders
	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit:
		orderReq.Price(order.Market.FormatPrice(order.Price))
	case types.OrderTypeMarket:
		// Because our order.Quantity unit is base coin, so we indicate the target currency to Base.
		if order.Side == types.SideTypeBuy {
			orderReq.Size(order.Market.FormatQuantity(order.Quantity))
			orderReq.TargetCurrency(okexapi.TargetCurrencyBase)
		} else {
			orderReq.Size(order.Market.FormatQuantity(order.Quantity))
			orderReq.TargetCurrency(okexapi.TargetCurrencyQuote)
		}
	}

	orderType, err := toLocalOrderType(order.Type)
	if err != nil {
		return nil, err
	}

	switch order.TimeInForce {
	case types.TimeInForceFOK:
		orderReq.OrderType(okexapi.OrderTypeFOK)
	case types.TimeInForceIOC:
		orderReq.OrderType(okexapi.OrderTypeIOC)
	default:
		orderReq.OrderType(orderType)
	}

	if err := placeOrderLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("place order rate limiter wait error: %w", err)
	}

	_, err = strconv.ParseInt(order.ClientOrderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("client order id should be numberic: %s, err: %w", order.ClientOrderID, err)
	}
	orderReq.ClientOrderID(order.ClientOrderID)

	orders, err := orderReq.Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(orders) != 1 {
		return nil, fmt.Errorf("unexpected length of order response: %v", orders)
	}

	orderRes, err := e.QueryOrder(ctx, types.OrderQuery{
		Symbol:        order.Symbol,
		OrderID:       orders[0].OrderID,
		ClientOrderID: orders[0].ClientOrderID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query order by id: %s, clientOrderId: %s, err: %w", orders[0].OrderID, orders[0].ClientOrderID, err)
	}

	return orderRes, nil

	// TODO: move this to batch place orders interface
	/*
		batchReq := e.client.TradeService.NewBatchPlaceOrderRequest()
		batchReq.Add(reqs...)
		orderHeads, err := batchReq.Do(ctx)
		if err != nil {
			return nil, err
		}

		for idx, orderHead := range orderHeads {
			orderID, err := strconv.ParseInt(orderHead.OrderID, 10, 64)
			if err != nil {
				return createdOrder, err
			}

			submitOrder := order[idx]
			createdOrder = append(createdOrder, types.Order{
				SubmitOrder:      submitOrder,
				Exchange:         types.ExchangeOKEx,
				OrderID:          uint64(orderID),
				Status:           types.OrderStatusNew,
				ExecutedQuantity: fixedpoint.Zero,
				IsWorking:        true,
				CreationTime:     types.Time(time.Now()),
				UpdateTime:       types.Time(time.Now()),
				IsMargin:         false,
				IsIsolated:       false,
			})
		}
	*/
}

// QueryOpenOrders retrieves the pending orders. The data returned is ordered by createdTime, and we utilized the
// `After` parameter to acquire all orders.
func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	instrumentID := toLocalSymbol(symbol)

	nextCursor := int64(0)
	for {
		if err := queryOpenOrderLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("query open orders rate limiter wait error: %w", err)
		}

		req := e.client.NewGetOpenOrdersRequest().
			InstrumentID(instrumentID).
			After(strconv.FormatInt(nextCursor, 10))
		openOrders, err := req.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query open orders: %w", err)
		}

		for _, o := range openOrders {
			o, err := orderDetailToGlobal(&o.OrderDetail)
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
		req.InstrumentID(toLocalSymbol(order.Symbol))
		req.OrderID(strconv.FormatUint(order.OrderID, 10))
		if len(order.ClientOrderID) > 0 {
			_, err := strconv.ParseInt(order.ClientOrderID, 10, 64)
			if err != nil {
				return fmt.Errorf("client order id should be numberic: %s, err: %w", order.ClientOrderID, err)
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
	return NewStream(e.client, e)
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	if err := marketDataLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	intervalParam, err := toLocalInterval(interval)
	if err != nil {
		return nil, fmt.Errorf("fail to get interval: %w", err)
	}

	req := e.client.NewCandlesticksRequest(toLocalSymbol(symbol))
	req.Bar(intervalParam)

	if options.StartTime != nil {
		req.After(options.StartTime.Unix())
	}

	if options.EndTime != nil {
		req.Before(options.EndTime.Unix())
	}

	candles, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var klines []types.KLine
	for _, candle := range candles {
		klines = append(klines, types.KLine{
			Exchange:    types.ExchangeOKEx,
			Symbol:      symbol,
			Interval:    interval,
			Open:        candle.Open,
			High:        candle.High,
			Low:         candle.Low,
			Close:       candle.Close,
			Closed:      true,
			Volume:      candle.Volume,
			QuoteVolume: candle.VolumeInCurrency,
			StartTime:   types.Time(candle.Time),
			EndTime:     types.Time(candle.Time.Add(interval.Duration() - time.Millisecond)),
		})
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
	req.InstrumentID(toLocalSymbol(q.Symbol)).
		OrderID(q.OrderID).
		ClientOrderID(q.ClientOrderID)

	var order *okexapi.OrderDetails
	order, err := req.Do(ctx)

	if err != nil {
		return nil, err
	}

	return toGlobalOrder(order)
}

// QueryOrderTrades quires order trades can query trades in last 3 months.
func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) (trades []types.Trade, err error) {
	if len(q.ClientOrderID) != 0 {
		log.Warn("!!!OKEX EXCHANGE API NOTICE!!! Okex does not support searching for trades using OrderClientId.")
	}

	req := e.client.NewGetTransactionHistoryRequest()
	if len(q.Symbol) != 0 {
		req.InstrumentID(toLocalSymbol(q.Symbol))
	}

	if len(q.OrderID) != 0 {
		req.OrderID(q.OrderID)
	}

	if err := queryTradeLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("order trade rate limiter wait error: %w", err)
	}
	response, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query order trades, err: %w", err)
	}

	for _, trade := range response {
		trades = append(trades, tradeToGlobal(trade))
	}

	return trades, nil
}

/*
QueryClosedOrders can query closed orders in last 3 months, there are no time interval limitations, as long as until >= since.
Please Use lastOrderID as cursor, only return orders later than that order, that order is not included.
If you want to query all orders within a large time range (e.g. total orders > 100), we recommend using batch.ClosedOrderBatchQuery.

** since and until are inclusive, you can include the lastTradeId as well. **
*/
func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
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

	res, err := e.client.NewGetOrderHistoryRequest().
		InstrumentID(toLocalSymbol(symbol)).
		StartTime(since).
		EndTime(until).
		Limit(defaultQueryLimit).
		Before(strconv.FormatUint(lastOrderID, 10)).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to call get order histories error: %w", err)
	}

	for _, order := range res {
		o, err2 := orderDetailToGlobal(&order)
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

/*
QueryTrades can query trades in last 3 months, there are no time interval limitations, as long as end_time >= start_time.
okx does not provide an API to query by trade ID, so we use the bill ID to do it. The trades result is ordered by timestamp.

REMARK: If your start time is 90 days earlier, we will update it to now - 90 days.
** StartTime and EndTime are inclusive. **
** StartTime and EndTime cannot exceed 90 days.  **
** StartTime, EndTime, FromTradeId can be used together. **

If you want to query all trades within a large time range (e.g. total orders > 100), we recommend using batch.TradeBatchQuery.
*/
func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	if symbol == "" {
		return nil, ErrSymbolRequired
	}

	req := e.client.NewGetTransactionHistoryRequest().InstrumentID(toLocalSymbol(symbol))

	limit := options.Limit
	req.Limit(uint64(limit))
	if limit > defaultQueryLimit || limit <= 0 {
		log.Infof("limit is exceeded default limit %d or zero, got: %d, use default limit", defaultQueryLimit, limit)
		req.Limit(defaultQueryLimit)
	}

	var newStartTime time.Time
	if options.StartTime != nil {
		newStartTime = *options.StartTime
		if time.Since(newStartTime) > maxHistoricalDataQueryPeriod {
			newStartTime = time.Now().Add(-maxHistoricalDataQueryPeriod)
			log.Warnf("!!!OKX EXCHANGE API NOTICE!!! The trade API cannot query data beyond 90 days from the current date, update %s -> %s", *options.StartTime, newStartTime)
		}
		req.StartTime(newStartTime.UTC())
	}

	if options.EndTime != nil {
		if options.EndTime.Before(newStartTime) {
			return nil, fmt.Errorf("end time %s before start %s", *options.EndTime, newStartTime)
		}
		if options.EndTime.Sub(newStartTime) > maxHistoricalDataQueryPeriod {
			return nil, fmt.Errorf("start time %s and end time %s cannot greater than 90 days", newStartTime, options.EndTime)
		}
		req.EndTime(options.EndTime.UTC())
	}
	req.Before(strconv.FormatUint(options.LastTradeID, 10))

	if err := queryTradeLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query trades rate limiter wait error: %w", err)
	}

	response, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades, err: %w", err)
	}

	for _, trade := range response {
		trades = append(trades, tradeToGlobal(trade))
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
