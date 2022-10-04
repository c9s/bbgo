package ftx

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/ftx/ftxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	restEndpoint       = "https://ftx.com"
	defaultHTTPTimeout = 15 * time.Second
)

var logger = logrus.WithField("exchange", "ftx")

// POST https://ftx.com/api/orders 429, Success: false, err: Do not send more than 2 orders on this market per 200ms
var requestLimit = rate.NewLimiter(rate.Every(220*time.Millisecond), 2)

var marketDataLimiter = rate.NewLimiter(rate.Every(500*time.Millisecond), 2)

//go:generate go run generate_symbol_map.go

type Exchange struct {
	client *ftxapi.RestClient

	key, secret             string
	subAccount              string
	restEndpoint            *url.URL
	orderAmountReduceFactor fixedpoint.Value
}

type MarketTicker struct {
	Market types.Market
	Price  fixedpoint.Value
	Ask    fixedpoint.Value
	Bid    fixedpoint.Value
	Last   fixedpoint.Value
}

type MarketMap map[string]MarketTicker

// FTX does not have broker ID
const spotBrokerID = "BBGO"

func newSpotClientOrderID(originalID string) (clientOrderID string) {
	prefix := "x-" + spotBrokerID
	prefixLen := len(prefix)

	if originalID != "" {
		// try to keep the whole original client order ID if user specifies it.
		if prefixLen+len(originalID) > 32 {
			return originalID
		}

		clientOrderID = prefix + originalID
		return clientOrderID
	}

	clientOrderID = uuid.New().String()
	clientOrderID = prefix + clientOrderID
	if len(clientOrderID) > 32 {
		return clientOrderID[0:32]
	}

	return clientOrderID
}

func NewExchange(key, secret string, subAccount string) *Exchange {
	u, err := url.Parse(restEndpoint)
	if err != nil {
		panic(err)
	}

	client := ftxapi.NewClient()
	client.Auth(key, secret, subAccount)
	return &Exchange{
		client:       client,
		restEndpoint: u,
		key:          key,
		// pragma: allowlist nextline secret
		secret:                  secret,
		subAccount:              subAccount,
		orderAmountReduceFactor: fixedpoint.One,
	}
}

func (e *Exchange) newRest() *restRequest {
	r := newRestRequest(&http.Client{Timeout: defaultHTTPTimeout}, e.restEndpoint).Auth(e.key, e.secret)
	if len(e.subAccount) > 0 {
		r.SubAccount(e.subAccount)
	}
	return r
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeFTX
}

func (e *Exchange) PlatformFeeCurrency() string {
	return toGlobalCurrency("FTT")
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.key, e.secret, e.subAccount, e)
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	markets, err := e._queryMarkets(ctx)
	if err != nil {
		return nil, err
	}
	marketMap := types.MarketMap{}
	for k, v := range markets {
		marketMap[k] = v.Market
	}
	return marketMap, nil
}

func (e *Exchange) _queryMarkets(ctx context.Context) (MarketMap, error) {
	req := e.client.NewGetMarketsRequest()
	ftxMarkets, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := MarketMap{}
	for _, m := range ftxMarkets {
		symbol := toGlobalSymbol(m.Name)
		symbolMap[symbol] = m.Name

		mkt2 := MarketTicker{
			Market: types.Market{
				Symbol:      symbol,
				LocalSymbol: m.Name,
				// The max precision is length(DefaultPow). For example, currently fixedpoint.DefaultPow
				// is 1e8, so the max precision will be 8.
				PricePrecision:  m.PriceIncrement.NumFractionalDigits(),
				VolumePrecision: m.SizeIncrement.NumFractionalDigits(),
				QuoteCurrency:   toGlobalCurrency(m.QuoteCurrency),
				BaseCurrency:    toGlobalCurrency(m.BaseCurrency),
				// FTX only limit your order by `MinProvideSize`, so I assign zero value to unsupported fields:
				// MinNotional, MinAmount, MaxQuantity, MinPrice and MaxPrice.
				MinNotional: fixedpoint.Zero,
				MinAmount:   fixedpoint.Zero,
				MinQuantity: m.MinProvideSize,
				MaxQuantity: fixedpoint.Zero,
				StepSize:    m.SizeIncrement,
				MinPrice:    fixedpoint.Zero,
				MaxPrice:    fixedpoint.Zero,
				TickSize:    m.PriceIncrement,
			},
			Price: m.Price,
			Bid:   m.Bid,
			Ask:   m.Ask,
			Last:  m.Last,
		}
		markets[symbol] = mkt2
	}
	return markets, nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {

	req := e.client.NewGetAccountRequest()
	ftxAccount, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	a := &types.Account{
		TotalAccountValue: ftxAccount.TotalAccountValue,
	}

	balances, err := e.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}

	a.UpdateBalances(balances)
	return a, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	balanceReq := e.client.NewGetBalancesRequest()
	ftxBalances, err := balanceReq.Do(ctx)
	if err != nil {
		return nil, err
	}

	var balances = make(types.BalanceMap)
	for _, r := range ftxBalances {
		currency := toGlobalCurrency(r.Coin)
		balances[currency] = types.Balance{
			Currency:  currency,
			Available: r.Free,
			Locked:    r.Total.Sub(r.Free),
		}
	}

	return balances, nil
}

// DefaultFeeRates returns the FTX Tier 1 fee
// See also https://help.ftx.com/hc/en-us/articles/360024479432-Fees
func (e *Exchange) DefaultFeeRates() types.ExchangeFee {
	return types.ExchangeFee{
		MakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.020), // 0.020%
		TakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.070), // 0.070%
	}
}

// SetModifyOrderAmountForFee protects the limit buy orders by reducing amount with taker fee.
// The amount is recalculated before submit: submit_amount = original_amount / (1 + taker_fee_rate) .
// This prevents balance exceeding error while closing position without spot margin enabled.
func (e *Exchange) SetModifyOrderAmountForFee(feeRate types.ExchangeFee) {
	e.orderAmountReduceFactor = fixedpoint.One.Add(feeRate.TakerFeeRate)
}

// resolution field in api
// window length in seconds. options: 15, 60, 300, 900, 3600, 14400, 86400, or any multiple of 86400 up to 30*86400
var supportedIntervals = map[types.Interval]int{
	types.Interval1m:  1 * 60,
	types.Interval5m:  5 * 60,
	types.Interval15m: 15 * 60,
	types.Interval1h:  60 * 60,
	types.Interval4h:  60 * 60 * 4,
	types.Interval1d:  60 * 60 * 24,
	types.Interval3d:  60 * 60 * 24 * 3,
}

func (e *Exchange) SupportedInterval() map[types.Interval]int {
	return supportedIntervals
}

func (e *Exchange) IsSupportedInterval(interval types.Interval) bool {
	return isIntervalSupportedInKLine(interval)
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	var klines []types.KLine

	// the fetch result is from newest to oldest
	// currentEnd = until
	// endTime := currentEnd.Add(interval.Duration())
	klines, err := e._queryKLines(ctx, symbol, interval, options)
	if err != nil {
		return nil, err
	}

	klines = types.SortKLinesAscending(klines)
	return klines, nil
}

func (e *Exchange) _queryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	if !isIntervalSupportedInKLine(interval) {
		return nil, fmt.Errorf("interval %s is not supported", interval.String())
	}

	if err := marketDataLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	// assign limit to a default value since ftx has the limit
	if options.Limit == 0 {
		options.Limit = 500
	}

	// if the time range exceed the ftx valid time range, we need to adjust the endTime
	if options.StartTime != nil && options.EndTime != nil {
		rangeDuration := options.EndTime.Sub(*options.StartTime)
		estimatedCount := rangeDuration / interval.Duration()

		if options.Limit != 0 && uint64(estimatedCount) > uint64(options.Limit) {
			endTime := options.StartTime.Add(interval.Duration() * time.Duration(options.Limit))
			options.EndTime = &endTime
		}
	}

	resp, err := e.newRest().marketRequest.HistoricalPrices(ctx, toLocalSymbol(symbol), interval, int64(options.Limit), options.StartTime, options.EndTime)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("ftx returns failure")
	}

	var klines []types.KLine
	for _, r := range resp.Result {
		globalKline, err := toGlobalKLine(symbol, interval, r)
		if err != nil {
			return nil, err
		}
		klines = append(klines, globalKline)
	}

	return klines, nil
}

func isIntervalSupportedInKLine(interval types.Interval) bool {
	_, ok := supportedIntervals[interval]
	return ok
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	tradeIDs := make(map[uint64]struct{})
	lastTradeID := options.LastTradeID

	req := e.client.NewGetFillsRequest()
	req.Market(toLocalSymbol(symbol))

	if options.StartTime != nil {
		req.StartTime(*options.StartTime)
	} else if options.EndTime != nil {
		req.EndTime(*options.EndTime)
	}

	req.Order("asc")
	fills, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(fills, func(i, j int) bool {
		return fills[i].Time.Before(fills[j].Time)
	})

	var trades []types.Trade
	symbol = strings.ToUpper(symbol)
	for _, fill := range fills {
		if _, ok := tradeIDs[fill.TradeId]; ok {
			continue
		}

		if options.StartTime != nil && fill.Time.Before(*options.StartTime) {
			continue
		}

		if options.EndTime != nil && fill.Time.After(*options.EndTime) {
			continue
		}

		if fill.TradeId <= lastTradeID {
			continue
		}

		tradeIDs[fill.TradeId] = struct{}{}
		lastTradeID = fill.TradeId

		t, err := toGlobalTrade(fill)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}

	return trades, nil
}

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	if until == (time.Time{}) {
		until = time.Now()
	}
	if since.After(until) {
		return nil, fmt.Errorf("invalid query deposit history time range, since: %+v, until: %+v", since, until)
	}
	asset = TrimUpperString(asset)

	resp, err := e.newRest().DepositHistory(ctx, since, until, 0)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("ftx returns failure")
	}
	sort.Slice(resp.Result, func(i, j int) bool {
		return resp.Result[i].Time.Before(resp.Result[j].Time.Time)
	})
	for _, r := range resp.Result {
		d, err := toGlobalDeposit(r)
		if err != nil {
			return nil, err
		}
		if d.Asset == asset && !since.After(d.Time.Time()) && !until.Before(d.Time.Time()) {
			allDeposits = append(allDeposits, d)
		}
	}
	return
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	// TODO: currently only support limit and market order
	// TODO: support time in force
	so := order
	if err := requestLimit.Wait(ctx); err != nil {
		logrus.WithError(err).Error("rate limit error")
	}

	orderType, err := toLocalOrderType(so.Type)
	if err != nil {
		logrus.WithError(err).Error("type error")
	}

	submitQuantity := so.Quantity
	switch orderType {
	case ftxapi.OrderTypeLimit, ftxapi.OrderTypeStopLimit:
		submitQuantity = so.Quantity.Div(e.orderAmountReduceFactor)
	}

	req := e.client.NewPlaceOrderRequest()
	req.Market(toLocalSymbol(TrimUpperString(so.Symbol)))
	req.OrderType(orderType)
	req.Side(ftxapi.Side(TrimLowerString(string(so.Side))))
	req.Size(submitQuantity)

	switch so.Type {
	case types.OrderTypeLimit, types.OrderTypeLimitMaker:
		req.Price(so.Price)

	}

	if so.Type == types.OrderTypeLimitMaker {
		req.PostOnly(true)
	}

	if so.TimeInForce == types.TimeInForceIOC {
		req.Ioc(true)
	}

	req.ClientID(newSpotClientOrderID(so.ClientOrderID))

	or, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to place order %+v: %w", so, err)
	}

	globalOrder, err := toGlobalOrderNew(*or)
	return &globalOrder, err
}

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
	if err != nil {
		return nil, err
	}

	req := e.client.NewGetOrderStatusRequest(uint64(orderID))
	ftxOrder, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	o, err := toGlobalOrderNew(*ftxOrder)
	if err != nil {
		return nil, err
	}

	return &o, err
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	// TODO: invoke open trigger orders

	req := e.client.NewGetOpenOrdersRequest(toLocalSymbol(symbol))
	ftxOrders, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, ftxOrder := range ftxOrders {
		o, err := toGlobalOrderNew(ftxOrder)
		if err != nil {
			return orders, err
		}

		orders = append(orders, o)
	}
	return orders, nil
}

// symbol, since and until are all optional. FTX can only query by order created time, not updated time.
// FTX doesn't support lastOrderID, so we will query by the time range first, and filter by the lastOrderID.
func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	symbol = TrimUpperString(symbol)

	req := e.client.NewGetOrderHistoryRequest(toLocalSymbol(symbol))

	if since != (time.Time{}) {
		req.StartTime(since)
	} else if until != (time.Time{}) {
		req.EndTime(until)
	}

	ftxOrders, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(ftxOrders, func(i, j int) bool {
		return ftxOrders[i].CreatedAt.Before(ftxOrders[j].CreatedAt)
	})

	for _, ftxOrder := range ftxOrders {
		switch ftxOrder.Status {
		case ftxapi.OrderStatusOpen, ftxapi.OrderStatusNew:
			continue
		}

		o, err := toGlobalOrderNew(ftxOrder)
		if err != nil {
			return orders, err
		}

		orders = append(orders, o)
	}
	return orders, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	for _, o := range orders {
		if err := requestLimit.Wait(ctx); err != nil {
			logrus.WithError(err).Error("rate limit error")
		}

		var resp *ftxapi.APIResponse
		var err error
		if len(o.ClientOrderID) > 0 {
			req := e.client.NewCancelOrderByClientOrderIdRequest(o.ClientOrderID)
			resp, err = req.Do(ctx)
		} else {
			req := e.client.NewCancelOrderRequest(strconv.FormatUint(o.OrderID, 10))
			resp, err = req.Do(ctx)
		}

		if err != nil {
			return err
		}

		if !resp.Success {
			return fmt.Errorf("cancel order failed: %s", resp.Result)
		}
	}
	return nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	ticketMap, err := e.QueryTickers(ctx, symbol)
	if err != nil {
		return nil, err
	}

	if ticker, ok := ticketMap[symbol]; ok {
		return &ticker, nil
	}
	return nil, fmt.Errorf("ticker %s not found", symbol)
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	var tickers = make(map[string]types.Ticker)

	markets, err := e._queryMarkets(ctx)
	if err != nil {
		return nil, err
	}

	m := make(map[string]struct{})
	for _, s := range symbol {
		m[toGlobalSymbol(s)] = struct{}{}
	}

	rest := e.newRest()
	for k, v := range markets {

		// if we provide symbol as condition then we only query the gieven symbol ,
		// or we should query "ALL" symbol in the market.
		if _, ok := m[toGlobalSymbol(k)]; len(symbol) != 0 && !ok {
			continue
		}

		if err := requestLimit.Wait(ctx); err != nil {
			logrus.WithError(err).Errorf("order rate limiter wait error")
		}

		// ctx context.Context, market string, interval types.Interval, limit int64, start, end time.Time
		now := time.Now()
		since := now.Add(time.Duration(-1) * time.Hour)
		until := now
		prices, err := rest.marketRequest.HistoricalPrices(ctx, v.Market.LocalSymbol, types.Interval1h, 1, &since, &until)
		if err != nil || !prices.Success || len(prices.Result) == 0 {
			continue
		}

		lastCandle := prices.Result[0]
		tickers[toGlobalSymbol(k)] = types.Ticker{
			Time:   lastCandle.StartTime.Time,
			Volume: lastCandle.Volume,
			Last:   v.Last,
			Open:   lastCandle.Open,
			High:   lastCandle.High,
			Low:    lastCandle.Low,
			Buy:    v.Bid,
			Sell:   v.Ask,
		}
	}

	return tickers, nil
}

func (e *Exchange) Transfer(ctx context.Context, coin string, size float64, destination string) (string, error) {
	payload := TransferPayload{
		Coin:        coin,
		Size:        size,
		Source:      e.subAccount,
		Destination: destination,
	}
	resp, err := e.newRest().Transfer(ctx, payload)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("ftx returns transfer failure")
	}
	return resp.Result.String(), nil
}
