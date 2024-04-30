package retry

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

func QueryTradesUntilSuccessful(
	ctx context.Context, ex types.ExchangeTradeHistoryService, symbol string, q *types.TradeQueryOptions,
) (trades []types.Trade, err error) {
	var op = func() (err2 error) {
		trades, err2 = ex.QueryTrades(ctx, symbol, q)
		for _, trade := range trades {
			if trade.FeeProcessing {
				return fmt.Errorf("trade fee of #%d (order #%d) is still processing", trade.ID, trade.OrderID)
			}
		}
		return err2
	}

	err = GeneralBackoff(ctx, op)
	return trades, err
}

func QueryTradesUntilSuccessfulLite(
	ctx context.Context, ex types.ExchangeTradeHistoryService, symbol string, q *types.TradeQueryOptions,
) (trades []types.Trade, err error) {
	var op = func() (err2 error) {
		trades, err2 = ex.QueryTrades(ctx, symbol, q)
		for _, trade := range trades {
			if trade.FeeProcessing {
				return fmt.Errorf("trade fee of #%d (order #%d) is still processing", trade.ID, trade.OrderID)
			}
		}
		return err2
	}

	err = GeneralLiteBackoff(ctx, op)
	return trades, err
}
