package batch

import (
	"context"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
)

type ClosedOrderBatchQuery struct {
	types.ExchangeTradeHistoryService
}

func (q *ClosedOrderBatchQuery) Query(ctx context.Context, symbol string, startTime, endTime time.Time, lastOrderID uint64, opts ...Option) (c chan types.Order, errC chan error) {
	query := &AsyncTimeRangedBatchQuery{
		Type: types.Order{},
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return retry.QueryClosedOrdersUntilSuccessfulLite(
				ctx,
				q.ExchangeTradeHistoryService,
				symbol,
				startTime,
				endTime,
				lastOrderID,
			)
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(types.Order).CreationTime)
		},
		ID: func(obj interface{}) string {
			order := obj.(types.Order)
			if order.OrderID > lastOrderID {
				lastOrderID = order.OrderID
			}
			return strconv.FormatUint(order.OrderID, 10)
		},
		JumpIfEmpty: 30 * 24 * time.Hour,
	}

	for _, opt := range opts {
		opt(query)
	}

	c = make(chan types.Order, 100)
	errC = query.Query(ctx, c, startTime, endTime)
	return c, errC
}
