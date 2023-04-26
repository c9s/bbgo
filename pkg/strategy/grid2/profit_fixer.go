package grid2

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitFixer struct {
	symbol         string
	grid           *Grid
	historyService types.ExchangeTradeHistoryService

	logger logrus.FieldLogger
}

func newProfitFixer(grid *Grid, symbol string, historyService types.ExchangeTradeHistoryService) *ProfitFixer {
	return &ProfitFixer{
		symbol:         symbol,
		grid:           grid,
		historyService: historyService,
		logger:         logrus.StandardLogger(),
	}
}

func (f *ProfitFixer) SetLogger(logger logrus.FieldLogger) {
	f.logger = logger
}

// Fix fixes the total quote profit of the given grid
func (f *ProfitFixer) Fix(parent context.Context, since, until time.Time, initialOrderID uint64, profitStats *GridProfitStats) error {
	// reset profit
	profitStats.TotalQuoteProfit = fixedpoint.Zero
	profitStats.ArbitrageCount = 0

	defer f.logger.Infof("profitFixer: done")

	if profitStats.Since != nil && !profitStats.Since.IsZero() && profitStats.Since.Before(since) {
		f.logger.Infof("profitFixer: profitStats.since %s is earlier than the given since %s, setting since to %s", profitStats.Since, since, profitStats.Since)
		since = *profitStats.Since
	}

	ctx, cancel := context.WithTimeout(parent, 15*time.Minute)
	defer cancel()

	q := &batch.ClosedOrderBatchQuery{ExchangeTradeHistoryService: f.historyService}
	orderC, errC := q.Query(ctx, f.symbol, since, until, initialOrderID)

	defer func() {
		f.logger.Infof("profitFixer: fixed profitStats=%#v", profitStats)
	}()

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

			f.logger.Debugf("profitFixer: filledSellOrder=%#v", order)
		}
	}
}
