package ftx

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	restEndpoint       = "https://ftx.com"
	defaultHTTPTimeout = 15 * time.Second
)

var Logger = logrus.WithField("exchange", "ftx")

type Exchange struct {
	rest        *restRequest
	key, secret string
}

func NewExchange(key, secret string, subAccount string) *Exchange {
	u, err := url.Parse(restEndpoint)
	if err != nil {
		panic(err)
	}
	rest := newRestRequest(&http.Client{Timeout: defaultHTTPTimeout}, u).Auth(key, secret)
	if subAccount != "" {
		rest.SubAccount(subAccount)
	}
	return &Exchange{
		rest:   rest,
		key:    key,
		secret: secret,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeFTX
}

func (e *Exchange) PlatformFeeCurrency() string {
	panic("implement me")
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.key, e.secret)
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	panic("implement me")
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	panic("implement me")
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	resp, err := e.rest.Balances(ctx)
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
	panic("implement me")
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
	panic("implement me")
}

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	panic("implement me")
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	panic("implement me")
}

func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	panic("implement me")
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	panic("implement me")
}
