package batch

import (
	"context"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/types"
)

type BinanceFuturesIncomeHistoryService interface {
	QueryFuturesIncomeHistory(ctx context.Context, symbol string, incomeType binanceapi.FuturesIncomeType, startTime, endTime *time.Time) ([]binanceapi.FuturesIncome, error)
}

type BinanceFuturesIncomeBatchQuery struct {
	BinanceFuturesIncomeHistoryService
}

func (e *BinanceFuturesIncomeBatchQuery) Query(ctx context.Context, symbol string, incomeType binanceapi.FuturesIncomeType, startTime, endTime time.Time) (c chan binanceapi.FuturesIncome, errC chan error) {
	query := &AsyncTimeRangedBatchQuery{
		Type:        types.MarginInterest{},
		Limiter:     rate.NewLimiter(rate.Every(3*time.Second), 1),
		JumpIfEmpty: time.Hour * 24 * 30,
		Q: func(startTime, endTime time.Time) (interface{}, error) {
			return e.QueryFuturesIncomeHistory(ctx, symbol, incomeType, &startTime, &endTime)
		},
		T: func(obj interface{}) time.Time {
			return time.Time(obj.(binanceapi.FuturesIncome).Time)
		},
		ID: func(obj interface{}) string {
			interest := obj.(binanceapi.FuturesIncome)
			return interest.Time.String()
		},
	}

	c = make(chan binanceapi.FuturesIncome, 100)
	errC = query.Query(ctx, c, startTime, endTime)
	return c, errC
}
