package batch

import (
	"context"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"
)

type RewardBatchQuery struct {
	Service types.ExchangeRewardService
}

func (q *RewardBatchQuery) Query(ctx context.Context, startTime, endTime time.Time) (c chan types.Reward, errC chan error) {
	query := &AsyncTimeRangedBatchQuery{
		Type:    types.Reward{},
		Limiter: rate.NewLimiter(rate.Every(5*time.Second), 2),
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return q.Service.QueryRewards(ctx, startTime)
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(types.Reward).CreatedAt)
		},
		ID: func(obj interface{}) string {
			return obj.(types.Reward).UUID
		},
	}

	c = make(chan types.Reward, 500)
	errC = query.Query(ctx, c, startTime, endTime)
	return c, errC
}
