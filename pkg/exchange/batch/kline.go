package batch

import (
	"context"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"
)

type KLineBatchQuery struct {
	types.Exchange
}

func (e *KLineBatchQuery) Query(ctx context.Context, symbol string, interval types.Interval, startTime, endTime time.Time) (c chan types.KLine, errC chan error) {
	query := &AsyncTimeRangedBatchQuery{
		Type:    types.KLine{},
		Limiter: rate.NewLimiter(rate.Every(5*time.Second), 2),
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return e.Exchange.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
				StartTime: &startTime,
				EndTime:   &endTime,
			})
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(types.KLine).StartTime).UTC()
		},
		ID: func(obj interface{}) string {
			kline := obj.(types.KLine)
			return kline.StartTime.String()
		},
	}

	c = make(chan types.KLine, 100)
	errC = query.Query(ctx, c, startTime, endTime)
	return c, errC
}
