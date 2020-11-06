package backtest

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

type Exchange struct {
	sourceExchange types.ExchangeName
	publicExchange types.Exchange
	srv            *service.BacktestService
	startTime      time.Time

	closedOrders []types.SubmitOrder
	openOrders   []types.SubmitOrder
}

func NewExchange(sourceExchange types.ExchangeName, srv *service.BacktestService, startTime time.Time) *Exchange {
	ex, err := newPublicExchange(sourceExchange)
	if err != nil {
		panic(err)
	}

	return &Exchange{
		sourceExchange: sourceExchange,
		publicExchange: ex,
		srv:            srv,
		startTime:      startTime,
	}
}

func (e *Exchange) NewStream() types.Stream {
	// TODO: return the stream and feed the data
	return &Stream{}
}

func (e Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	panic("implement me")
}

func (e Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	panic("implement me")
}

func (e Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	panic("implement me")
}

func (e Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	panic("implement me")
}

func (e Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	panic("implement me")
}

func (e Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	panic("implement me")
}

func (e Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	return e.publicExchange.QueryKLines(ctx, symbol, interval, options)
}

func (e Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	// we don't need query trades for backtest
	return nil, nil
}

func (e Exchange) Name() types.ExchangeName {
	return e.publicExchange.Name()
}

func (e Exchange) PlatformFeeCurrency() string {
	return e.publicExchange.PlatformFeeCurrency()
}

func (e Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	return e.publicExchange.QueryMarkets(ctx)
}

func (e Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	return nil, nil
}

func (e Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
	return nil, nil
}

func newPublicExchange(sourceExchange types.ExchangeName) (types.Exchange, error) {
	switch sourceExchange {
	case types.ExchangeBinance:
		return binance.New("", ""), nil
	case types.ExchangeMax:
		return max.New("", ""), nil
	}

	return nil, errors.Errorf("exchange %s is not supported", sourceExchange)
}
