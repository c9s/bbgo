package ftx

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	restEndpoint       = "https://ftx.com"
	defaultHTTPTimeout = 15 * time.Second
)

var logger = logrus.WithField("exchange", "ftx")

//go:generate go run generate_symbol_map.go

type Exchange struct {
	key, secret  string
	subAccount   string
	restEndpoint *url.URL
}

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

	return &Exchange{
		restEndpoint: u,
		key:          key,
		secret:       secret,
		subAccount:   subAccount,
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
	resp, err := e.newRest().Markets(ctx)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("ftx returns querying markets failure")
	}

	markets := types.MarketMap{}
	for _, m := range resp.Result {
		symbol := toGlobalSymbol(m.Name)
		symbolMap[symbol] = m.Name

		market := types.Market{
			Symbol:      symbol,
			LocalSymbol: m.Name,

			// The max precision is length(DefaultPow). For example, currently fixedpoint.DefaultPow
			// is 1e8, so the max precision will be 8.
			PricePrecision:  fixedpoint.NumFractionalDigits(fixedpoint.NewFromFloat(m.PriceIncrement)),
			VolumePrecision: fixedpoint.NumFractionalDigits(fixedpoint.NewFromFloat(m.SizeIncrement)),
			QuoteCurrency:   toGlobalCurrency(m.QuoteCurrency),
			BaseCurrency:    toGlobalCurrency(m.BaseCurrency),
			// FTX only limit your order by `MinProvideSize`, so I assign zero value to unsupported fields:
			// MinNotional, MinAmount, MaxQuantity, MinPrice and MaxPrice.
			MinNotional: 0,
			MinAmount:   0,
			MinQuantity: m.MinProvideSize,
			MaxQuantity: 0,
			StepSize:    m.SizeIncrement,
			MinPrice:    0,
			MaxPrice:    0,
			TickSize:    m.PriceIncrement,
		}
		markets[symbol] = market
	}
	return markets, nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	resp, err := e.newRest().Account(ctx)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("ftx returns querying balances failure")
	}

	a := &types.Account{
		MakerCommission:   fixedpoint.NewFromFloat(resp.Result.MakerFee),
		TakerCommission:   fixedpoint.NewFromFloat(resp.Result.TakerFee),
		TotalAccountValue: fixedpoint.NewFromFloat(resp.Result.TotalAccountValue),
	}

	balances, err := e.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}
	a.UpdateBalances(balances)

	return a, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	resp, err := e.newRest().Balances(ctx)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("ftx returns querying balances failure")
	}
	var balances = make(types.BalanceMap)
	for _, r := range resp.Result {
		balances[toGlobalCurrency(r.Coin)] = types.Balance{
			Currency:  toGlobalCurrency(r.Coin),
			Available: fixedpoint.NewFromFloat(r.Free),
			Locked:    fixedpoint.NewFromFloat(r.Total).Sub(fixedpoint.NewFromFloat(r.Free)),
		}
	}

	return balances, nil
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	var since, until time.Time
	if options.StartTime != nil {
		since = *options.StartTime
	}
	if options.EndTime != nil {
		until = *options.EndTime
	} else {
		until = time.Now()
	}
	if since.After(until) {
		return nil, fmt.Errorf("invalid query klines time range, since: %+v, until: %+v", since, until)
	}
	if !isIntervalSupportedInKLine(interval) {
		return nil, fmt.Errorf("interval %s is not supported", interval.String())
	}
	resp, err := e.newRest().HistoricalPrices(ctx, toLocalSymbol(symbol), interval, int64(options.Limit), since, until)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("ftx returns failure")
	}

	var kline []types.KLine
	for _, r := range resp.Result {
		globalKline, err := toGlobalKLine(symbol, interval, r)
		if err != nil {
			return nil, err
		}
		kline = append(kline, globalKline)
	}
	return kline, nil
}

var supportedInterval = map[int]struct{}{
	15:    {},
	60:    {},
	300:   {},
	900:   {},
	3600:  {},
	14400: {},
	86400: {},
}

func isIntervalSupportedInKLine(interval types.Interval) bool {
	_, ok := supportedInterval[interval.Minutes()*60]
	return ok
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	var since, until time.Time
	if options.StartTime != nil {
		since = *options.StartTime
	}
	if options.EndTime != nil {
		until = *options.EndTime
	} else {
		until = time.Now()
	}

	if since.After(until) {
		return nil, fmt.Errorf("invalid query trades time range, since: %+v, until: %+v", since, until)
	}

	if options.Limit == 1 {
		// FTX doesn't provide pagination api, so we have to split the since/until time range into small slices, and paginate ourselves.
		// If the limit is 1, we always get the same data from FTX.
		return nil, fmt.Errorf("limit can't be 1 which can't be used in pagination")
	}
	limit := options.Limit
	if limit == 0 {
		limit = 200
	}

	tradeIDs := make(map[int64]struct{})

	lastTradeID := options.LastTradeID
	var trades []types.Trade
	symbol = strings.ToUpper(symbol)

	for since.Before(until) {
		// DO not set limit to `1` since you will always get the same response.
		resp, err := e.newRest().Fills(ctx, toLocalSymbol(symbol), since, until, limit, true)
		if err != nil {
			return nil, err
		}
		if !resp.Success {
			return nil, fmt.Errorf("ftx returns failure")
		}

		sort.Slice(resp.Result, func(i, j int) bool {
			return resp.Result[i].TradeId < resp.Result[j].TradeId
		})

		for _, r := range resp.Result {
			// always update since to avoid infinite loop
			since = r.Time.Time

			if _, ok := tradeIDs[r.TradeId]; ok {
				continue
			}

			if r.TradeId <= lastTradeID || r.Time.Before(since) || r.Time.After(until) || r.Market != toLocalSymbol(symbol) {
				continue
			}
			tradeIDs[r.TradeId] = struct{}{}
			lastTradeID = r.TradeId

			t, err := toGlobalTrade(r)
			if err != nil {
				return nil, err
			}
			trades = append(trades, t)
		}

		if int64(len(resp.Result)) < limit {
			return trades, nil
		}
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

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (types.OrderSlice, error) {
	var createdOrders types.OrderSlice
	// TODO: currently only support limit and market order
	// TODO: support time in force
	for _, so := range orders {
		if so.TimeInForce != "GTC" && so.TimeInForce != "" {
			return createdOrders, fmt.Errorf("unsupported TimeInForce %s. only support GTC", so.TimeInForce)
		}
		or, err := e.newRest().PlaceOrder(ctx, PlaceOrderPayload{
			Market:     toLocalSymbol(TrimUpperString(so.Symbol)),
			Side:       TrimLowerString(string(so.Side)),
			Price:      so.Price,
			Type:       TrimLowerString(string(so.Type)),
			Size:       so.Quantity,
			ReduceOnly: false,
			IOC:        false,
			PostOnly:   false,
			ClientID:   newSpotClientOrderID(so.ClientOrderID),
		})
		if err != nil {
			return createdOrders, fmt.Errorf("failed to place order %+v: %w", so, err)
		}
		if !or.Success {
			return createdOrders, fmt.Errorf("ftx returns placing order failure")
		}
		globalOrder, err := toGlobalOrder(or.Result)
		if err != nil {
			return createdOrders, fmt.Errorf("failed to convert response to global order")
		}
		createdOrders = append(createdOrders, globalOrder)
	}
	return createdOrders, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	// TODO: invoke open trigger orders
	resp, err := e.newRest().OpenOrders(ctx, toLocalSymbol(symbol))
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("ftx returns querying open orders failure")
	}
	for _, r := range resp.Result {
		o, err := toGlobalOrder(r)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// symbol, since and until are all optional. FTX can only query by order created time, not updated time.
// FTX doesn't support lastOrderID, so we will query by the time range first, and filter by the lastOrderID.
func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	if until == (time.Time{}) {
		until = time.Now()
	}
	if since.After(until) {
		return nil, fmt.Errorf("invalid query closed orders time range, since: %+v, until: %+v", since, until)
	}

	symbol = TrimUpperString(symbol)
	limit := int64(100)
	hasMoreData := true
	s := since
	var lastOrder order
	for hasMoreData {
		resp, err := e.newRest().OrdersHistory(ctx, toLocalSymbol(symbol), s, until, limit)
		if err != nil {
			return nil, err
		}
		if !resp.Success {
			return nil, fmt.Errorf("ftx returns querying orders history failure")
		}

		sortByCreatedASC(resp.Result)

		for _, r := range resp.Result {
			// There may be more than one orders at the same time, so also have to check the ID
			if r.CreatedAt.Before(lastOrder.CreatedAt.Time) || r.ID == lastOrder.ID || r.Status != "closed" || r.ID < int64(lastOrderID) {
				continue
			}
			lastOrder = r
			o, err := toGlobalOrder(r)
			if err != nil {
				return nil, err
			}
			orders = append(orders, o)
		}
		hasMoreData = resp.HasMoreData
		// the start_time and end_time precision is second. There might be more than one orders within one second.
		s = lastOrder.CreatedAt.Add(-1 * time.Second)
	}
	return orders, nil
}

func sortByCreatedASC(orders []order) {
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].CreatedAt.Before(orders[j].CreatedAt.Time)
	})
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	for _, o := range orders {
		rest := e.newRest()
		if len(o.ClientOrderID) > 0 {
			if _, err := rest.CancelOrderByClientID(ctx, o.ClientOrderID); err != nil {
				return err
			}
			continue
		}
		if _, err := rest.CancelOrderByOrderID(ctx, o.OrderID); err != nil {
			return err
		}
	}
	return nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	panic("implement me")
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	panic("implement me")
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
