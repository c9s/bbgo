package batch

import (
	"context"
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

			orders, err := e.QueryClosedOrders(ctx, symbol, startTime, endTime, lastOrderID)
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

func (e KLineBatchQuery) Query(ctx context.Context, symbol string, interval types.Interval, startTime, endTime time.Time) (c chan types.KLine, errC chan error) {
	c = make(chan types.KLine, 1000)
	errC = make(chan error, 1)

	go func() {
		defer close(c)
		defer close(errC)

		for startTime.Before(endTime) {
			kLines, err := e.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
				StartTime: &startTime,
			})

			if err != nil {
				errC <- err
				return
			}

			if len(kLines) == 0 {
				return
			}

			for _, kline := range kLines {
				// ignore any kline before the given start time
				if kline.StartTime.Before(startTime) {
					continue
				}

				if kline.StartTime.After(endTime) {
					return
				}

				c <- kline
				startTime = kline.EndTime.Add(time.Millisecond)
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

			trades, err = e.Exchange.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
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
