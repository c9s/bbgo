package batch

import (
	"context"
	"strconv"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"
)

type MarginLiquidationBatchQuery struct {
	types.MarginHistory
}

func (e *MarginLiquidationBatchQuery) Query(ctx context.Context, startTime, endTime time.Time) (c chan types.MarginLiquidation, errC chan error) {
	query := &AsyncTimeRangedBatchQuery{
		Type:        types.MarginLiquidation{},
		Limiter:     rate.NewLimiter(rate.Every(5*time.Second), 2),
		JumpIfEmpty: time.Hour * 24 * 30,
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return e.QueryLiquidationHistory(ctx, &startTime, &endTime)
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(types.MarginLiquidation).UpdatedTime)
		},
		ID: func(obj interface{}) string {
			liquidation := obj.(types.MarginLiquidation)
			return strconv.FormatUint(liquidation.OrderID, 10)
		},
	}

	c = make(chan types.MarginLiquidation, 100)
	errC = query.Query(ctx, c, startTime, endTime)
	return c, errC
}
