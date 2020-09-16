package types

import (
	"context"
	"time"
)

type Exchange interface {
	QueryKLines(interval string, startFrom time.Time, endTo time.Time) []KLineOrWindow
	QueryTrades(symbol string, startFrom time.Time) []Trade

	SubmitOrder(ctx context.Context, order *SubmitOrder) error
}
