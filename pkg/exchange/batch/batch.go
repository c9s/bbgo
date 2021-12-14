package batch

import (
	"context"
	"github.com/pkg/errors"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"
)

type ClosedOrderBatchQuery struct {
	types.Exchange
}

func (e ClosedOrderBatchQuery) Query(ctx context.Context, symbol string, startTime, endTime time.Time, lastOrderID uint64) (c chan types.Order, errC chan error) {
	c = make(chan types.Order, 500)
	errC = make(chan error, 1)

	tradeHistoryService, ok := e.Exchange.(types.ExchangeTradeHistoryService)
	if !ok {
		// skip exchanges that does not support trading history services
		logrus.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing closed orders", e.Exchange.Name())
		return c, errC
	}

	go func() {
		limiter := rate.NewLimiter(rate.Every(5*time.Second), 2) // from binance (original 1200, use 1000 for safety)

		defer close(c)
		defer close(errC)

		orderIDs := make(map[uint64]struct{}, 500)
		if lastOrderID > 0 {
			orderIDs[lastOrderID] = struct{}{}
		}

		for startTime.Before(endTime) {
			if err := limiter.Wait(ctx); err != nil {
				logrus.WithError(err).Error("rate limit error")
			}

			logrus.Infof("batch querying %s closed orders %s <=> %s", symbol, startTime, endTime)

			orders, err := tradeHistoryService.QueryClosedOrders(ctx, symbol, startTime, endTime, lastOrderID)
			if err != nil {
				errC <- err
				return
			}

			if len(orders) == 0 || (len(orders) == 1 && orders[0].OrderID == lastOrderID) {
				return
			}

			for _, o := range orders {
				if _, ok := orderIDs[o.OrderID]; ok {
					continue
				}

				c <- o
				startTime = o.CreationTime.Time()
				lastOrderID = o.OrderID
				orderIDs[o.OrderID] = struct{}{}
			}
		}

	}()

	return c, errC
}

type KLineBatchQuery struct {
	types.Exchange
}

func (e KLineBatchQuery) Query(ctx context.Context, symbol string, interval types.Interval, startTime, endTime time.Time) (c chan []types.KLine, errC chan error) {
	c = make(chan []types.KLine, 1000)
	errC = make(chan error, 1)

	go func() {
		defer close(c)
		defer close(errC)

		tryQueryKlineTimes := 0

		var nowStartTime = startTime
		for nowStartTime.Before(endTime) {
			kLines, err := e.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
				StartTime: &nowStartTime,
				EndTime:   &endTime,
			})
			sort.Slice(kLines, func(i, j int) bool { return kLines[i].StartTime.Unix() < kLines[j].StartTime.Unix() })
			tryQueryKlineTimes++

			if err != nil {
				errC <- err
				return
			}

			if len(kLines) == 0 {
				return
			}
			const BatchSize = 200

			var batchKLines = make([]types.KLine, 0, BatchSize)
			for _, kline := range kLines {
				// ignore any kline before the given start time
				if nowStartTime.Unix() != startTime.Unix() && kline.StartTime.Unix() <= nowStartTime.Unix() {
					continue
				}

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

				//The issue is in FTX, prev endtime = next start time , so if add 1 ms , it would query forever.
				nowStartTime = kline.StartTime
				tryQueryKlineTimes = 0
			}

			if len(batchKLines) != 0 {
				c <- batchKLines
				batchKLines = nil
			}

			if tryQueryKlineTimes > 10 { // it means loop 10 times
				errC <- errors.Errorf("There's a dead loop in batch.go#Query , symbol: %s , interval: %s, startTime :%s ", symbol, interval, startTime.String())
				return
			}
		}
	}()

	return c, errC
}

type TradeBatchQuery struct {
	types.Exchange
}

func (e TradeBatchQuery) Query(ctx context.Context, symbol string, options *types.TradeQueryOptions) (c chan types.Trade, errC chan error) {
	c = make(chan types.Trade, 500)
	errC = make(chan error, 1)

	tradeHistoryService, ok := e.Exchange.(types.ExchangeTradeHistoryService)
	if !ok {
		// skip exchanges that does not support trading history services
		logrus.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing closed orders", e.Exchange.Name())
		return c, errC
	}

	var lastTradeID = options.LastTradeID

	go func() {
		limiter := rate.NewLimiter(rate.Every(5*time.Second), 2) // from binance (original 1200, use 1000 for safety)

		defer close(c)
		defer close(errC)

		var tradeKeys = map[types.TradeKey]struct{}{}

		for {
			if err := limiter.Wait(ctx); err != nil {
				logrus.WithError(err).Error("rate limit error")
			}

			logrus.Infof("querying %s trades from id=%d limit=%d", symbol, lastTradeID, options.Limit)

			var err error
			var trades []types.Trade

			trades, err = tradeHistoryService.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
				Limit:       options.Limit,
				LastTradeID: lastTradeID,
			})

			if err != nil {
				errC <- err
				return
			}

			if len(trades) == 0 {
				return
			} else if len(trades) == 1 {
				k := trades[0].Key()
				if _, exists := tradeKeys[k]; exists {
					return
				}
			}

			for _, t := range trades {
				key := t.Key()
				if _, ok := tradeKeys[key]; ok {
					logrus.Debugf("ignore duplicated trade: %+v", key)
					continue
				}

				lastTradeID = t.ID
				tradeKeys[key] = struct{}{}

				// ignore the first trade if last TradeID is given
				c <- t
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
				logrus.WithError(err).Error("rate limit error")
			}

			logrus.Infof("batch querying rewards %s <=> %s", startTime, endTime)

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
