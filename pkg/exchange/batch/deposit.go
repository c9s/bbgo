package batch

import (
	"context"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"
)

type DepositBatchQuery struct {
	types.ExchangeTransferService
}

func (e *DepositBatchQuery) Query(ctx context.Context, asset string, startTime, endTime time.Time) (c chan types.Deposit, errC chan error) {
	query := &AsyncTimeRangedBatchQuery{
		Type:        types.Deposit{},
		Limiter:     rate.NewLimiter(rate.Every(5*time.Second), 2),
		JumpIfEmpty: time.Hour * 24 * 80,
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return e.ExchangeTransferService.QueryDepositHistory(ctx, asset, startTime, endTime)
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(types.Deposit).Time)
		},
		ID: func(obj interface{}) string {
			deposit := obj.(types.Deposit)
			return deposit.TransactionID
		},
	}

	c = make(chan types.Deposit, 100)
	errC = query.Query(ctx, c, startTime, endTime)
	return c, errC
}
