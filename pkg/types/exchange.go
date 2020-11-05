package types

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type ExchangeName string

func (n ExchangeName) String() string {
	return string(n)
}

const (
	ExchangeMax     = ExchangeName("max")
	ExchangeBinance = ExchangeName("binance")
)

func ValidExchangeName(a string) (ExchangeName, error) {
	switch strings.ToLower(a) {
	case "max":
		return ExchangeMax, nil
	case "binance", "bn":
		return ExchangeBinance, nil
	}

	return "", errors.New("invalid exchange name")
}

type Exchange interface {
	Name() ExchangeName

	PlatformFeeCurrency() string

	NewStream() Stream

	QueryMarkets(ctx context.Context) (MarketMap, error)

	QueryAccount(ctx context.Context) (*Account, error)

	QueryAccountBalances(ctx context.Context) (BalanceMap, error)

	QueryKLines(ctx context.Context, symbol string, interval string, options KLineQueryOptions) ([]KLine, error)

	QueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) ([]Trade, error)

	QueryAveragePrice(ctx context.Context, symbol string) (float64, error)

	QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []Deposit, err error)

	QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []Withdraw, err error)

	SubmitOrders(ctx context.Context, orders ...SubmitOrder) (createdOrders OrderSlice, err error)

	QueryOpenOrders(ctx context.Context, symbol string) (orders []Order, err error)

	QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []Order, err error)

	CancelOrders(ctx context.Context, orders ...Order) error
}

type TradeQueryOptions struct {
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int64
	LastTradeID int64
}

type ExchangeBatchProcessor struct {
	Exchange
}

func (e ExchangeBatchProcessor) BatchQueryClosedOrders(ctx context.Context, symbol string, startTime, endTime time.Time, lastOrderID uint64) (c chan Order, errC chan error) {
	c = make(chan Order, 500)
	errC = make(chan error, 1)

	go func() {
		defer close(c)
		defer close(errC)

		orderIDs := make(map[uint64]struct{}, 500)
		if lastOrderID > 0 {
			orderIDs[lastOrderID] = struct{}{}
		}

		for startTime.Before(endTime) {
			limitedEndTime := startTime.Add(24 * time.Hour)
			orders, err := e.QueryClosedOrders(ctx, symbol, startTime, limitedEndTime, lastOrderID)
			if err != nil {
				errC <- err
				return
			}

			if len(orders) == 0 || (len(orders) == 1 && orders[0].OrderID == lastOrderID) {
				startTime = limitedEndTime
				continue
			}

			for _, o := range orders {
				if _, ok := orderIDs[o.OrderID]; ok {
					log.Infof("skipping duplicated order id: %d", o.OrderID)
					continue
				}

				c <- o
				startTime = o.CreationTime
				lastOrderID = o.OrderID
				orderIDs[o.OrderID] = struct{}{}
			}
		}

	}()

	return c, errC
}

func (e ExchangeBatchProcessor) BatchQueryKLines(ctx context.Context, symbol, interval string, startTime, endTime time.Time) (allKLines []KLine, err error) {
	for startTime.Before(endTime) {
		kLines, err := e.QueryKLines(ctx, symbol, interval, KLineQueryOptions{
			StartTime: &startTime,
			Limit:     1000,
		})

		if err != nil {
			return allKLines, err
		}

		for _, kline := range kLines {
			if kline.EndTime.After(endTime) {
				return allKLines, nil
			}

			allKLines = append(allKLines, kline)
			startTime = kline.EndTime
		}
	}

	return allKLines, err
}

func (e ExchangeBatchProcessor) BatchQueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) (c chan Trade, errC chan error) {
	c = make(chan Trade, 500)
	errC = make(chan error, 1)

	// last 7 days
	var startTime = time.Now().Add(-7 * 24 * time.Hour)
	if options.StartTime != nil {
		startTime = *options.StartTime
	}

	var lastTradeID = options.LastTradeID

	go func() {
		defer close(c)
		defer close(errC)

		for {
			log.Infof("querying %s trades from %s, limit=%d", symbol, startTime, options.Limit)

			trades, err := e.QueryTrades(ctx, symbol, &TradeQueryOptions{
				StartTime:   &startTime,
				Limit:       options.Limit,
				LastTradeID: lastTradeID,
			})
			if err != nil {
				errC <- err
				return
			}

			if len(trades) == 0 {
				break
			}

			if len(trades) == 1 && trades[0].ID == lastTradeID {
				break
			}

			log.Infof("returned %d trades", len(trades))

			startTime = trades[len(trades)-1].Time
			for _, t := range trades {
				// ignore the first trade if last TradeID is given
				if t.ID == lastTradeID {
					continue
				}

				c <- t
				lastTradeID = t.ID
			}
		}
	}()

	return c, errC
}
