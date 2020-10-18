package backtest

import (
	"context"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type Stream struct {
	types.StandardStream
}

func (s *Stream) Connect(ctx context.Context) error {
	return nil
}

func (s *Stream) Close() error {
	return nil
}

type Trader struct {
	// Context is trading Context
	Context                 *bbgo.Context
	SourceKLines            []types.KLine
	ProfitAndLossCalculator *pnl.AverageCostCalculator

	doneOrders    []types.SubmitOrder
	pendingOrders []types.SubmitOrder
}

func (trader *Trader) SubmitOrder(ctx context.Context, order types.SubmitOrder) {
	trader.pendingOrders = append(trader.pendingOrders, order)
}
