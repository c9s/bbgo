package service

import (
	"context"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

type MarginService struct {
	DB *sqlx.DB
}

func (s *MarginService) Sync(ctx context.Context, ex types.Exchange, asset string, startTime time.Time) error {
	api, ok := ex.(types.MarginHistory)
	if !ok {
		return nil
	}

	marginExchange, ok := ex.(types.MarginExchange)
	if !ok {
		return nil
	}

	marginSettings := marginExchange.GetMarginSettings()
	if !marginSettings.IsMargin {
		return nil
	}

	tasks := []SyncTask{
		{
			Select: SelectLastMarginLoans(ex.Name(), 100),
			Type:   types.MarginLoan{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginLoanBatchQuery{
					MarginHistory: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginLoan).Time.Time()
			},
			ID: func(obj interface{}) string {
				return strconv.FormatUint(obj.(types.MarginLoan).TransactionID, 10)
			},
			LogInsert: true,
		},
		{
			Select: SelectLastMarginRepays(ex.Name(), 100),
			Type:   types.MarginRepay{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginRepayBatchQuery{
					MarginHistory: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginRepay).Time.Time()
			},
			ID: func(obj interface{}) string {
				return strconv.FormatUint(obj.(types.MarginRepay).TransactionID, 10)
			},
			LogInsert: true,
		},
		{
			Select: SelectLastMarginInterests(ex.Name(), 100),
			Type:   types.MarginInterest{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginInterestBatchQuery{
					MarginHistory: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginInterest).Time.Time()
			},
			ID: func(obj interface{}) string {
				m := obj.(types.MarginInterest)
				return m.Asset + m.IsolatedSymbol + strconv.FormatInt(m.Time.UnixMilli(), 10)
			},
			LogInsert: true,
		},
		{
			Select: SelectLastMarginLiquidations(ex.Name(), 100),
			Type:   types.MarginLiquidation{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginLiquidationBatchQuery{
					MarginHistory: api,
				}
				return query.Query(ctx, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginLiquidation).UpdatedTime.Time()
			},
			ID: func(obj interface{}) string {
				m := obj.(types.MarginLiquidation)
				return strconv.FormatUint(m.OrderID, 10)
			},
			LogInsert: true,
		},
	}

	for _, sel := range tasks {
		if err := sel.execute(ctx, s.DB, startTime); err != nil {
			return err
		}
	}

	return nil
}

func SelectLastMarginLoans(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_loans").
		Where(sq.Eq{"exchange": ex}).
		OrderBy("time DESC").
		Limit(limit)
}

func SelectLastMarginRepays(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_repays").
		Where(sq.Eq{"exchange": ex}).
		OrderBy("time DESC").
		Limit(limit)
}

func SelectLastMarginInterests(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_interests").
		Where(sq.Eq{"exchange": ex}).
		OrderBy("time DESC").
		Limit(limit)
}

func SelectLastMarginLiquidations(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_liquidations").
		Where(sq.Eq{"exchange": ex}).
		OrderBy("time DESC").
		Limit(limit)
}
