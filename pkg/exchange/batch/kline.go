package batch

import (
	"context"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

type KLineBatchQuery struct {
	types.Exchange

	// MaxRetries is the number of retries of a failed page query, 0 means the default (32)
	MaxRetries uint64
}

func (e *KLineBatchQuery) Query(ctx context.Context, symbol string, interval types.Interval, startTime, endTime time.Time) (c chan types.KLine, errC chan error) {
	query := &AsyncTimeRangedBatchQuery{
		Type:       types.KLine{},
		Limiter:    nil, // the rate limiter is handled in the exchange query method
		MaxRetries: e.MaxRetries,
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return e.Exchange.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
				StartTime: &startTime,
				EndTime:   &endTime,
			})
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(types.KLine).StartTime)
		},
		ID: func(obj interface{}) string {
			kline := obj.(types.KLine)
			return strconv.FormatInt(kline.StartTime.UnixMilli(), 10)
		},
	}

	c = make(chan types.KLine, 3000)
	errC = query.Query(ctx, c, startTime, endTime)
	return c, errC
}
