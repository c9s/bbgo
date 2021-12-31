package batch

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/types"
)

type TradeBatchQuery struct {
	types.Exchange
}

func (e TradeBatchQuery) Query(ctx context.Context, symbol string, options *types.TradeQueryOptions) (c chan types.Trade, errC chan error) {
	c = make(chan types.Trade, 500)
	errC = make(chan error, 1)

	tradeHistoryService, ok := e.Exchange.(types.ExchangeTradeHistoryService)
	if !ok {
		close(errC)
		close(c)
		// skip exchanges that does not support trading history services
		logrus.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing closed orders (TradeBatchQuery.Query)", e.Exchange.Name())
		return c, errC
	}

	if options.StartTime == nil {

		errC <- errors.New("start time is required for syncing trades")
		close(errC)
		close(c)
		return c, errC
	}

	var lastTradeID = options.LastTradeID
	var startTime = *options.StartTime
	var endTime = *options.EndTime


	go func() {
		limiter := rate.NewLimiter(rate.Every(5*time.Second), 2) // from binance (original 1200, use 1000 for safety)

		defer close(c)
		defer close(errC)

		var tradeKeys = map[types.TradeKey]struct{}{}

		for startTime.Before(endTime) {
			if err := limiter.Wait(ctx); err != nil {
				logrus.WithError(err).Error("rate limit error")
			}

			logrus.Infof("querying %s trades from id=%d limit=%d between %s <=> %s", symbol, lastTradeID, options.Limit, startTime, endTime)

			var err error
			var trades []types.Trade

			trades, err = tradeHistoryService.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
				StartTime:   options.StartTime,
				LastTradeID: lastTradeID,
			})

			// sort trades by time in ascending order
			sort.Slice(trades, func(i, j int) bool {
				return trades[i].Time.Before(time.Time(trades[j].Time))
			})

			if err != nil {
				errC <- err
				return
			}

			// if all trades are duplicated or empty, we end the batch query
			if len(trades) == 0 {
				return
			} else if len(trades) > 0 {
				allExists := true
				for _, td := range trades {
					k := td.Key()
					if _, exists := tradeKeys[k]; !exists {
						allExists = false
						break
					}
				}
				if allExists {
					return
				}
			}

			for _, td := range trades {
				key := td.Key()

				logrus.Debugf("checking trade key: %v trade: %+v", key, td)

				if _, ok := tradeKeys[key]; ok {
					logrus.Debugf("ignore duplicated trade: %+v", key)
					continue
				}

				lastTradeID = td.ID
				startTime = time.Time(td.Time)
				tradeKeys[key] = struct{}{}

				// ignore the first trade if last TradeID is given
				c <- td
			}
		}
	}()

	return c, errC
}
