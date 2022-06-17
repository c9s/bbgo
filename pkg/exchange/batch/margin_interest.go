package batch

import (
	"context"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"
)

type MarginInterestBatchQuery struct {
	types.MarginHistory
}

func (e *MarginInterestBatchQuery) Query(ctx context.Context, asset string, startTime, endTime time.Time) (c chan types.MarginInterest, errC chan error) {
	query := &AsyncTimeRangedBatchQuery{
		Type:        types.MarginInterest{},
		Limiter:     rate.NewLimiter(rate.Every(5*time.Second), 2),
		JumpIfEmpty: time.Hour * 24 * 30,
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return e.QueryInterestHistory(ctx, asset, &startTime, &endTime)
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(types.MarginInterest).Time)
		},
		ID: func(obj interface{}) string {
			interest := obj.(types.MarginInterest)
			return interest.Time.String()
		},
	}

	c = make(chan types.MarginInterest, 100)
	errC = query.Query(ctx, c, startTime, endTime)
	return c, errC
}
