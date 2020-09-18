package types

import (
	"context"
	"time"
)

type Exchange interface {
	QueryKLines(ctx context.Context, symbol string, interval string, options KLineQueryOptions) ([]KLine, error)
	QueryTrades(ctx context.Context, symbol string, options *TradeQueryOptions) ([]Trade, error)

	SubmitOrder(ctx context.Context, order *SubmitOrder) error
}

type TradeQueryOptions struct {
	StartTime   *time.Time
	EndTime     *time.Time
	Limit       int
	LastTradeID int64
}

