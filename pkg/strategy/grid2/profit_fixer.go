package grid2

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitFixer struct {
	symbol         string
	grid           *Grid
	historyService types.ExchangeTradeHistoryService
}

func newProfitFixer(grid *Grid, symbol string, historyService types.ExchangeTradeHistoryService) *ProfitFixer {
	return &ProfitFixer{
		symbol:         symbol,
		grid:           grid,
		historyService: historyService,
	}
}

// Fix fixes the total quote profit of the given grid
func (f *ProfitFixer) Fix(ctx context.Context, since, until time.Time, initialOrderID uint64, profitStats *GridProfitStats) error {
	profitStats.TotalQuoteProfit = fixedpoint.Zero
	profitStats.ArbitrageCount = 0

	q := &batch.ClosedOrderBatchQuery{ExchangeTradeHistoryService: f.historyService}
	orderC, errC := q.Query(ctx, f.symbol, since, until, initialOrderID)

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}

			return ctx.Err()

		case order, ok := <-orderC:
			if !ok {
				return <-errC
			}

			if !f.grid.HasPrice(order.Price) {
				continue
			}

			if profitStats.InitialOrderID == 0 || order.OrderID < profitStats.InitialOrderID {
				profitStats.InitialOrderID = order.OrderID
			}

			if profitStats.Since == nil || profitStats.Since.IsZero() || order.CreationTime.Time().Before(*profitStats.Since) {
				ct := order.CreationTime.Time()
				profitStats.Since = &ct
			}

			if order.Status != types.OrderStatusFilled {
				continue
			}

			if order.Type != types.OrderTypeLimit {
				continue
			}

			if order.Side != types.SideTypeSell {
				continue
			}

			quoteProfit := order.Quantity.Mul(f.grid.Spread)
			profitStats.TotalQuoteProfit = profitStats.TotalQuoteProfit.Add(quoteProfit)
			profitStats.ArbitrageCount++

			log.Debugf("profitFixer: filledSellOrder=%#v", order)
			log.Debugf("profitFixer: profitStats=%#v", profitStats)
		}
	}
}
