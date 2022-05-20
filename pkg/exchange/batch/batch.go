package batch

import (
	"context"
	"sort"
	"time"

	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("component", "batch")

type KLineBatchQuery struct {
	types.Exchange
}

func (e KLineBatchQuery) Query(ctx context.Context, symbol string, interval types.Interval, startTime, endTime time.Time) (c chan []types.KLine, errC chan error) {
	c = make(chan []types.KLine, 1000)
	errC = make(chan error, 1)

	go func() {
		defer close(c)
		defer close(errC)

		var tryQueryKlineTimes = 0
		for startTime.Before(endTime) {
			log.Debugf("batch query klines %s %s %s <=> %s", symbol, interval, startTime, endTime)

			kLines, err := e.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
				StartTime: &startTime,
				EndTime:   &endTime,
			})

			if err != nil {
				errC <- err
				return
			}

			// ensure the kline is in the right order
			sort.Slice(kLines, func(i, j int) bool {
				return kLines[i].StartTime.Unix() < kLines[j].StartTime.Unix()
			})

			if len(kLines) == 0 {
				return
			}

			tryQueryKlineTimes++
			const BatchSize = 200

			var batchKLines = make([]types.KLine, 0, BatchSize)
			for _, kline := range kLines {
				// ignore any kline before the given start time of the batch query
				if kline.StartTime.Before(startTime) {
					continue
				}

				// if there is a kline after the endTime of the batch query, it means the data is out of scope, we should exit
				if kline.StartTime.After(endTime) || kline.EndTime.After(endTime) {
					if len(batchKLines) != 0 {
						c <- batchKLines
						batchKLines = nil
					}
					return
				}

				batchKLines = append(batchKLines, kline)

				if len(batchKLines) == BatchSize {
					c <- batchKLines
					batchKLines = nil
				}

				// The issue is in FTX, prev endtime = next start time , so if add 1 ms , it would query forever.
				// (above comment was written by @tony1223)
				startTime = kline.EndTime.Time()
				tryQueryKlineTimes = 0
			}

			// push the rest klines in the buffer
			if len(batchKLines) > 0 {
				c <- batchKLines
				batchKLines = nil
			}

			if tryQueryKlineTimes > 10 { // it means loop 10 times
				errC <- errors.Errorf("there's a dead loop in batch.go#Query , symbol: %s , interval: %s, startTime:%s ", symbol, interval, startTime.String())
				return
			}
		}
	}()

	return c, errC
}

type RewardBatchQuery struct {
	Service types.ExchangeRewardService
}

func (q *RewardBatchQuery) Query(ctx context.Context, startTime, endTime time.Time) (c chan types.Reward, errC chan error) {
	c = make(chan types.Reward, 500)
	errC = make(chan error, 1)

	go func() {
		limiter := rate.NewLimiter(rate.Every(5*time.Second), 2) // from binance (original 1200, use 1000 for safety)

		defer close(c)
		defer close(errC)

		lastID := ""
		rewardKeys := make(map[string]struct{}, 500)

		for startTime.Before(endTime) {
			if err := limiter.Wait(ctx); err != nil {
				log.WithError(err).Error("rate limit error")
			}

			log.Infof("batch querying rewards %s <=> %s", startTime, endTime)

			rewards, err := q.Service.QueryRewards(ctx, startTime)
			if err != nil {
				errC <- err
				return
			}

			// empty data
			if len(rewards) == 0 {
				return
			}

			// there is no new data
			if len(rewards) == 1 && rewards[0].UUID == lastID {
				return
			}

			newCnt := 0
			for _, o := range rewards {
				if _, ok := rewardKeys[o.UUID]; ok {
					continue
				}

				if o.CreatedAt.Time().After(endTime) {
					// stop batch query
					return
				}

				newCnt++
				c <- o
				rewardKeys[o.UUID] = struct{}{}
			}

			if newCnt == 0 {
				return
			}

			end := len(rewards) - 1
			startTime = rewards[end].CreatedAt.Time()
			lastID = rewards[end].UUID
		}

	}()

	return c, errC
}
