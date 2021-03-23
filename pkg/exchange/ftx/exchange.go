package ftx

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	restEndpoint       = "https://ftx.com"
	defaultHTTPTimeout = 15 * time.Second
)

var logger = logrus.WithField("exchange", "ftx")

type Exchange struct {
	key, secret  string
	subAccount   string
	restEndpoint *url.URL
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
	return NewStream(e.key, e.secret)
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

		market := types.Market{
			Symbol: symbol,
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
		MakerCommission: fixedpoint.NewFromFloat(resp.Result.MakerFee),
		TakerCommission: fixedpoint.NewFromFloat(resp.Result.TakerFee),
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
	panic("implement me")
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	panic("implement me")
}

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	if err = verifySinceUntil(since, until); err != nil {
		return nil, err
	}
	asset = TrimUpperString(asset)

	resp, err := e.newRest().DepositHistory(ctx, since, until, 0)
	if err != nil {
		return nil, err
	}
	sort.Slice(resp.Result, func(i, j int) bool {
		return resp.Result[i].Time.Before(resp.Result[j].Time)
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

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
	panic("implement me")
}

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (types.OrderSlice, error) {
	var createdOrders types.OrderSlice
	// TODO: currently only support limit and market order
	// TODO: support time in force
	for _, so := range orders {
		if so.TimeInForce != "GTC" {
			return createdOrders, fmt.Errorf("unsupported TimeInForce %s. only support GTC", so.TimeInForce)
		}
		or, err := e.newRest().PlaceOrder(ctx, PlaceOrderPayload{
			Market:     TrimUpperString(so.Symbol),
			Side:       TrimLowerString(string(so.Side)),
			Price:      so.Price,
			Type:       TrimLowerString(string(so.Type)),
			Size:       so.Quantity,
			ReduceOnly: false,
			IOC:        false,
			PostOnly:   false,
			ClientID:   so.ClientOrderID,
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
	resp, err := e.newRest().OpenOrders(ctx, symbol)
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
	if err := verifySinceUntil(since, until); err != nil {
		return nil, err
	}

	symbol = TrimUpperString(symbol)
	limit := int64(100)
	hasMoreData := true
	s := since
	var lastOrder order
	for hasMoreData {
		resp, err := e.newRest().OrdersHistory(ctx, symbol, s, until, limit)
		if err != nil {
			return nil, err
		}
		if !resp.Success {
			return nil, fmt.Errorf("ftx returns querying orders history failure")
		}

		sortByCreatedASC(resp.Result)

		for _, r := range resp.Result {
			// There may be more than one orders at the same time, so also have to check the ID
			if r.CreatedAt.Before(lastOrder.CreatedAt) || r.ID == lastOrder.ID || r.Status != "closed" || r.ID < int64(lastOrderID) {
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
		return orders[i].CreatedAt.Before(orders[j].CreatedAt)
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

func verifySinceUntil(since, until time.Time) error {
	if since.After(until) {
		return fmt.Errorf("since can't be greater than until")
	}
	if since == (time.Time{}) {
		return fmt.Errorf("since not found")
	}
	if until == (time.Time{}) {
		return fmt.Errorf("until not found")
	}
	return nil
}
