package batch

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

var closedErrChan = make(chan error)

func init() {
	close(closedErrChan)
}

type TradeBatchQuery struct {
	types.ExchangeTradeHistoryService
}

func (e TradeBatchQuery) Query(
	ctx context.Context, symbol string, options *types.TradeQueryOptions, opts ...Option,
) (c chan types.Trade, errC chan error) {
	if options.EndTime == nil {
		now := time.Now()
		options.EndTime = &now
	}

	query := &AsyncTimeRangedBatchQuery{
		Type: types.Trade{},
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return e.ExchangeTradeHistoryService.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
				StartTime:   &startTime,
				EndTime:     &endTime,
				Limit:       options.Limit,
				LastTradeID: options.LastTradeID,
			})
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(types.Trade).Time)
		},
		ID: func(obj interface{}) string {
			trade := obj.(types.Trade)
			if trade.ID > options.LastTradeID {
				options.LastTradeID = trade.ID
			}

			return trade.Key().String()
		},
		JumpIfEmpty: 24*time.Hour - 5*time.Minute, // exchange may not have trades in the last 24 hours
	}

	for _, opt := range opts {
		opt(query)
	}

	c = make(chan types.Trade, 100)
	errC = query.Query(ctx, c, *options.StartTime, *options.EndTime)
	return c, errC
}
