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
	orderRateLimiter  = rate.NewLimiter(rate.Every(300*time.Millisecond), 5)

	queryMarketLimiter      = rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
	queryTickerLimiter      = rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
	queryTickersLimiter     = rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
	queryAccountLimiter     = rate.NewLimiter(rate.Every(200*time.Millisecond), 5)
	placeOrderLimiter       = rate.NewLimiter(rate.Every(30*time.Millisecond), 30)
	batchCancelOrderLimiter = rate.NewLimiter(rate.Every(5*time.Millisecond), 200)
	queryOpenOrderLimiter   = rate.NewLimiter(rate.Every(30*time.Millisecond), 30)
)

const (
	ID = "okex"

	// PlatformToken is the platform currency of OKEx, pre-allocate static string here
	PlatformToken = "OKB"

	defaultQueryLimit = 100
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
			o, err := openOrderToGlobal(&o)
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
	return NewStream(e.client)
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

// Query order trades can query trades in last 3 months.
func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
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

	if err := orderRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("order rate limiter wait error: %w", err)
	}
	response, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query order trades, err: %w", err)
	}

	var trades []types.Trade
	var errs error
	for _, trade := range response {
		res, err := toGlobalTrade(&trade)
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

/*
QueryClosedOrders can query closed orders in last 3 months, there are no time interval limitations, as long as until >= since.
Please Use lastOrderID as cursor, only return orders later than that order, that order is not included.
If you want to query orders by time range, please just pass since and until.
If you want to query by cursor, please pass lastOrderID.
Because it gets the correct response even when you pass all parameters with the right time interval and invalid lastOrderID, like 0.
Time interval boundary unit is second.
since is inclusive, ex. order created in 1694155903, get response if query since 1694155903, get empty if query since 1694155904
until is not inclusive, ex. order created in 1694155903, get response if query until 1694155904, get empty if query until 1694155903
*/
func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error) {
	if symbol == "" {
		return nil, ErrSymbolRequired
	}

	if err := orderRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query closed order rate limiter wait error: %w", err)
	}

	var lastOrder string
	if lastOrderID <= 0 {
		lastOrder = ""
	} else {
		lastOrder = strconv.FormatUint(lastOrderID, 10)
	}

	res, err := e.client.NewGetOrderHistoryRequest().
		InstrumentID(toLocalSymbol(symbol)).
		StartTime(since).
		EndTime(until).
		Limit(defaultQueryLimit).
		Before(lastOrder).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to call get order histories error: %w", err)
	}

	var orders []types.Order

	for _, order := range res {
		o, err2 := toGlobalOrder(&order)
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
OKEX do not provide api to query by tradeID, So use /api/v5/trade/orders-history-archive as its official site do.
If you want to query trades by time range, please just pass start_time and end_time.
Because it gets the correct response even when you pass all parameters with the right time interval and invalid LastTradeID, like 0.
No matter how you pass parameter, QueryTrades return descending order.
If you query time period 3 months earlier with start time and end time, will return [] empty slice
But If you query time period 3 months earlier JUST with start time, will return like start with 3 months ago.
*/
func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	if symbol == "" {
		return nil, ErrSymbolRequired
	}

	if options.LastTradeID > 0 {
		log.Warn("!!!OKEX EXCHANGE API NOTICE!!! Okex does not support searching for trades using TradeId.")
	}

	req := e.client.NewGetTransactionHistoryRequest().InstrumentID(toLocalSymbol(symbol))

	limit := uint64(options.Limit)
	if limit > defaultQueryLimit || limit <= 0 {
		limit = defaultQueryLimit
		req.Limit(defaultQueryLimit)
		log.Debugf("limit is exceeded default limit %d or zero, got: %d, use default limit", defaultQueryLimit, options.Limit)
	} else {
		req.Limit(limit)
	}

	if err := orderRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query trades rate limiter wait error: %w", err)
	}

	var err error
	var response []okexapi.OrderDetails
	if options.StartTime == nil && options.EndTime == nil {
		return nil, fmt.Errorf("StartTime and EndTime are required parameter!")
	} else { // query by time interval
		if options.StartTime != nil {
			req.StartTime(*options.StartTime)
		}
		if options.EndTime != nil {
			req.EndTime(*options.EndTime)
		}

		var billID = "" // billId should be emtpy, can't be 0
		for {           // pagenation should use "after" (earlier than)
			res, err := req.
				After(billID).
				Do(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to call get order histories error: %w", err)
			}
			response = append(response, res...)
			if len(res) != int(limit) {
				break
			}
			billID = strconv.Itoa(int(res[limit-1].BillID))
		}
	}

	trades, err := toGlobalTrades(response)
	if err != nil {
		return nil, fmt.Errorf("failed to trans order detail to trades error: %w", err)
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
