package retry

import (
	"context"
	"errors"
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

func QueryTradesUntilSuccessful(
	ctx context.Context, ex types.ExchangeTradeHistoryService, symbol string, q *types.TradeQueryOptions,
) (trades []types.Trade, err error) {
	var stopOnErr error
	var op = func() (err2 error) {
		trades, err2 = ex.QueryTrades(ctx, symbol, q)
		for _, trade := range trades {
			if trade.FeeProcessing {
				return fmt.Errorf("order #%d(%s): trade fee of #%d is still processing", trade.OrderID, trade.OrderUUID, trade.ID)
			}
		}

		if errors.Is(err2, context.DeadlineExceeded) {
			// return nil to stop retrying
			stopOnErr = fmt.Errorf("retry query trades stopped on %w", err2)
			return nil
		}
		return err2
	}

	err = GeneralBackoff(ctx, op)
	if stopOnErr != nil {
		err = stopOnErr
	}
	return trades, err
}

func QueryTradesUntilSuccessfulLite(
	ctx context.Context, ex types.ExchangeTradeHistoryService, symbol string, q *types.TradeQueryOptions,
) (trades []types.Trade, err error) {
	var stopOnErr error
	var op = func() (err2 error) {
		trades, err2 = ex.QueryTrades(ctx, symbol, q)
		for _, trade := range trades {
			if trade.FeeProcessing {
				return fmt.Errorf("order #%d(%s): trade fee of #%d is still processing", trade.OrderID, trade.OrderUUID, trade.ID)
			}
		}
		if errors.Is(err2, context.DeadlineExceeded) {
			// return nil to stop retrying
			stopOnErr = fmt.Errorf("retry query trades lite stopped on %w", err2)
			return nil
		}
		return err2
	}

	err = GeneralLiteBackoff(ctx, op)
	if stopOnErr != nil {
		err = stopOnErr
	}
	return trades, err
}
