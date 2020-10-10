package types

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

type Exchange interface {
	PlatformFeeCurrency() string

	NewStream() Stream

	QueryAccount(ctx context.Context) (*Account, error)

	QueryAccountBalances(ctx context.Context) (BalanceMap, error)

	QueryKLines(ctx context.Context, symbol string, interval string, options KLineQueryOptions) ([]KLine, error)

	QueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) ([]Trade, error)

	SubmitOrder(ctx context.Context, order *SubmitOrder) error
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

func (e ExchangeBatchProcessor) BatchQueryKLines(ctx context.Context, symbol, interval string, startTime, endTime time.Time) (allKLines []KLine, err error) {
	for startTime.Before(endTime) {
		klines, err := e.QueryKLines(ctx, symbol, interval, KLineQueryOptions{
			StartTime: &startTime,
			Limit:     1000,
		})

		if err != nil {
			return nil, err
		}

		for _, kline := range klines {
			if kline.EndTime.After(endTime) {
				return allKLines, nil
			}

			allKLines = append(allKLines, kline)
			startTime = kline.EndTime
		}
	}

	return allKLines, err
}


func (e ExchangeBatchProcessor) BatchQueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) (allTrades []Trade, err error) {
	var startTime = time.Now().Add(-7 * 24 * time.Hour)
	if options.StartTime != nil {
		startTime = *options.StartTime
	}

	log.Infof("querying %s trades from %s", symbol, startTime)

	var lastTradeID = options.LastTradeID
	for {
		trades, err := e.QueryTrades(ctx, symbol, &TradeQueryOptions{
			StartTime:   &startTime,
			Limit:       options.Limit,
			LastTradeID: lastTradeID,
		})
		if err != nil {
			return allTrades, err
		}

		if len(trades) == 1 && trades[0].ID == lastTradeID {
			break
		}

		for _, t := range trades {
			// ignore the first trade if last TradeID is given
			if t.ID == lastTradeID {
				continue
			}

			allTrades = append(allTrades, t)
			lastTradeID = t.ID
		}
	}

	return allTrades, nil
}
