package batch

import (
	"context"
	"strconv"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"
)

type MarginRepayBatchQuery struct {
	types.MarginHistory
}

func (e *MarginRepayBatchQuery) Query(ctx context.Context, asset string, startTime, endTime time.Time) (c chan types.MarginRepay, errC chan error) {
	query := &AsyncTimeRangedBatchQuery{
		Type:        types.MarginRepay{},
		Limiter:     rate.NewLimiter(rate.Every(5*time.Second), 2),
		JumpIfEmpty: time.Hour * 24 * 30,
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return e.QueryRepayHistory(ctx, asset, &startTime, &endTime)
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(types.MarginRepay).Time)
		},
		ID: func(obj interface{}) string {
			loan := obj.(types.MarginRepay)
			return strconv.FormatUint(loan.TransactionID, 10)
		},
	}

	c = make(chan types.MarginRepay, 100)
	errC = query.Query(ctx, c, startTime, endTime)
	return c, errC
}
