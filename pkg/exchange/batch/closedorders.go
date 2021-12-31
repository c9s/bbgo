package batch

import (
	"context"
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
		defer close(c)
		defer close(errC)
		// skip exchanges that does not support trading history services
		logrus.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing closed orders (ClosedOrderBatchQuery.Query) ", e.Exchange.Name())
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
			for _, o := range orders {
				logrus.Infof("%+v", o)
			}

			if len(orders) == 0 {
				return
			} else if len(orders) > 0 {
				allExists := true
				for _, o := range orders {
					if _, exists := orderIDs[o.OrderID]; !exists {
						allExists = false
						break
					}
				}
				if allExists {
					return
				}
			}

			// sort orders by time in ascending order
			sort.Slice(orders, func(i, j int) bool {
				return orders[i].CreationTime.Before(time.Time(orders[j].CreationTime))
			})

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

