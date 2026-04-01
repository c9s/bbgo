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
	profitStats.DailyNumOfArbitrage = make(map[time.Time]int)
	profitStats.DailyProfit = make(map[time.Time]fixedpoint.Value)
	profitStats.DailyFee = make(map[time.Time]map[string]fixedpoint.Value)

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

	var err error
	keepRunning := true
	gridOrders := make(map[uint64]struct{})
	for keepRunning {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}

			return ctx.Err()

		case order, ok := <-orderC:
			if !ok {
				keepRunning = false
				err = <-errC
				continue
			}

			if !f.grid.HasPrice(order.Price) {
				continue
			}
			gridOrders[order.OrderID] = struct{}{}

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

			// Normalize to midnight UTC for daily aggregation
			orderTime := order.CreationTime.Time()
			dateKey := getDateKey(orderTime)
			profitStats.DailyNumOfArbitrage[dateKey]++
			profitStats.DailyProfit[dateKey] = profitStats.DailyProfit[dateKey].Add(quoteProfit)

			f.logger.Debugf("profitFixer: filledSellOrder=%#v", order)
		}
	}
	if err != nil {
		// query order error, skip fee aggregation and return error
		return err
	}
	// query trades to aggregate daily fee
	trades, err := f.queryTradesWithinTimeRange(ctx, since, until)
	if err != nil {
		f.logger.WithError(err).Warnf("profitFixer: failed to query trades for fee aggregation")
		return err
	}
	f.logger.Infof("profitFixer: collected %d trades", len(trades))
	for _, trade := range trades {
		if _, ok := gridOrders[trade.OrderID]; !ok {
			continue
		}
		dateKey := getDateKey(trade.Time.Time())
		if trade.Fee.Sign() <= 0 {
			continue
		}
		if profitStats.DailyFee[dateKey] == nil {
			profitStats.DailyFee[dateKey] = make(map[string]fixedpoint.Value)
		}
		profitStats.DailyFee[dateKey][trade.FeeCurrency] = profitStats.DailyFee[dateKey][trade.FeeCurrency].Add(trade.Fee)
	}
	return nil
}

// queryTradesWithinTimeRange queries all trades in the given time range.
func (f *ProfitFixer) queryTradesWithinTimeRange(ctx context.Context, since, until time.Time) ([]types.Trade, error) {
	tradeQuery := &batch.TradeBatchQuery{ExchangeTradeHistoryService: f.historyService}
	tradeC, errC := tradeQuery.Query(ctx, f.symbol, &types.TradeQueryOptions{
		StartTime: &since,
		EndTime:   &until,
	})

	var trades []types.Trade
	var err error
	keepRunning := true
	for keepRunning {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, nil
			}

			return nil, ctx.Err()

		case trade, ok := <-tradeC:
			if !ok {
				keepRunning = false
				err = <-errC
				continue
			}
			trades = append(trades, trade)
		}
	}

	return trades, err
}
