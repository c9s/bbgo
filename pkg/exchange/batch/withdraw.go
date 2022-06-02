package batch

import (
	"context"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"
)

type WithdrawBatchQuery struct {
	types.ExchangeTransferService
}

func (e *WithdrawBatchQuery) Query(ctx context.Context, asset string, startTime, endTime time.Time) (c chan types.Withdraw, errC chan error) {
	query := &AsyncTimeRangedBatchQuery{
		Type:        types.Withdraw{},
		Limiter:     rate.NewLimiter(rate.Every(5*time.Second), 2),
		JumpIfEmpty: time.Hour * 24 * 80,
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return e.ExchangeTransferService.QueryWithdrawHistory(ctx, asset, startTime, endTime)
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(types.Withdraw).ApplyTime)
		},
		ID: func(obj interface{}) string {
			withdraw := obj.(types.Withdraw)
			return withdraw.TransactionID
		},
	}

	c = make(chan types.Withdraw, 100)
	errC = query.Query(ctx, c, startTime, endTime)
	return c, errC
}
