package batch

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/types"
)

type ExchangeBatchProcessor struct {
	types.Exchange
}

func (e ExchangeBatchProcessor) BatchQueryClosedOrders(ctx context.Context, symbol string, startTime, endTime time.Time, lastOrderID uint64) (c chan types.Order, errC chan error) {
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
					logrus.Infof("skipping duplicated order id: %d", o.OrderID)
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

func (e ExchangeBatchProcessor) BatchQueryKLines(ctx context.Context, symbol string, interval types.Interval, startTime, endTime time.Time) (c chan types.KLine, errC chan error) {
	c = make(chan types.KLine, 1000)
	errC = make(chan error, 1)

	go func() {
		limiter := rate.NewLimiter(rate.Every(5*time.Second), 2) // from binance (original 1200, use 1000 for safety)

		defer close(c)
		defer close(errC)

		for startTime.Before(endTime) {
			if err := limiter.Wait(ctx); err != nil {
				logrus.WithError(err).Error("rate limit error")
			}

			kLines, err := e.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
				StartTime: &startTime,
				Limit:     1000,
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

				if kline.EndTime.After(endTime) {
					return
				}

				c <- kline
				startTime = kline.EndTime
			}
		}
	}()

	return c, errC
}

func (e ExchangeBatchProcessor) BatchQueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (c chan types.Trade, errC chan error) {
	c = make(chan types.Trade, 500)
	errC = make(chan error, 1)

	var lastTradeID = options.LastTradeID

	// last 7 days
	var startTime = time.Now().Add(-7 * 24 * time.Hour)
	if options.StartTime != nil {
		startTime = *options.StartTime
	}

	go func() {
		limiter := rate.NewLimiter(rate.Every(5*time.Second), 2) // from binance (original 1200, use 1000 for safety)

		defer close(c)
		defer close(errC)

		for {
			if err := limiter.Wait(ctx); err != nil {
				logrus.WithError(err).Error("rate limit error")
			}

			logrus.Infof("querying %s trades from %s, limit=%d", symbol, startTime, options.Limit)

			var err error
			var trades []types.Trade

			switch ex := e.Exchange.(type) {

			case *binance.Exchange:
				trades, err = ex.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
					StartTime: &startTime,
					Limit:     options.Limit,

					// only for MAX right now. binance is not using it, since we need to handle self-trade for binance
					LastTradeID: lastTradeID,
				})

			default:
				trades, err = ex.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
					StartTime: &startTime,
					Limit:     options.Limit,

					// only for MAX right now. binance is not using it, since we need to handle self-trade for binance
					LastTradeID: lastTradeID,
				})
			}

			if err != nil {
				errC <- err
				return
			}

			if len(trades) == 0 {
				break
			}

			logrus.Infof("returned %d trades", len(trades))

			// increase the window to the next time frame by adding 1 millisecond
			startTime = time.Time(trades[len(trades)-1].Time)
			for _, t := range trades {
				lastTradeID = t.ID

				// ignore the first trade if last TradeID is given
				c <- t
			}
		}
	}()

	return c, errC
}
