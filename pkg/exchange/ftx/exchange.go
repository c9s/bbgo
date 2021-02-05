package ftx

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

type Exchange struct {
}

func (e Exchange) Name() types.ExchangeName {
	panic("implement me")
}

func (e Exchange) PlatformFeeCurrency() string {
	panic("implement me")
}

func (e Exchange) NewStream() types.Stream {
	panic("implement me")
}

func (e Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	panic("implement me")
}

func (e Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	panic("implement me")
}

func (e Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	panic("implement me")
}

func (e Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	panic("implement me")
}

func (e Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	panic("implement me")
}

func (e Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	panic("implement me")
}

func (e Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
	panic("implement me")
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
