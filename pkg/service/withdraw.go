package service

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

type WithdrawService struct {
	DB *sqlx.DB
}

// Sync syncs the withdrawal records into db
func (s *WithdrawService) Sync(ctx context.Context, ex types.Exchange, startTime time.Time) error {
	isMargin, isFutures, isIsolated, _ := exchange.GetSessionAttributes(ex)
	if isMargin || isFutures || isIsolated {
		// only works in spot
		return nil
	}

	transferApi, ok := ex.(types.ExchangeTransferService)
	if !ok {
		return nil
	}

	tasks := []SyncTask{
		{
			Type:   types.Withdraw{},
			Select: SelectLastWithdraws(ex.Name(), 100),
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.WithdrawBatchQuery{
					ExchangeTransferService: transferApi,
				}
				return query.Query(ctx, "", startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.Withdraw).ApplyTime.Time()
			},
			ID: func(obj interface{}) string {
				withdraw := obj.(types.Withdraw)
				return withdraw.TransactionID
			},
			Filter: func(obj interface{}) bool {
				withdraw := obj.(types.Withdraw)
				if withdraw.Status == "rejected" {
					return false
				}

				if len(withdraw.TransactionID) == 0 {
					return false
				}

				return true
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

func SelectLastWithdraws(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("withdraws").
		Where(sq.And{
			sq.Eq{"exchange": ex},
		}).
		OrderBy("time DESC").
		Limit(limit)
}

func (s *WithdrawService) QueryLast(ex types.ExchangeName, limit int) ([]types.Withdraw, error) {
	sql := "SELECT * FROM `withdraws` WHERE `exchange` = :exchange ORDER BY `time` DESC LIMIT :limit"
	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"exchange": ex,
		"limit":    limit,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return s.scanRows(rows)
}

func (s *WithdrawService) Query(exchangeName types.ExchangeName) ([]types.Withdraw, error) {
	args := map[string]interface{}{
		"exchange": exchangeName,
	}
	sql := "SELECT * FROM `withdraws` WHERE `exchange` = :exchange ORDER BY `time` ASC"
	rows, err := s.DB.NamedQuery(sql, args)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
}

func (s *WithdrawService) scanRows(rows *sqlx.Rows) (withdraws []types.Withdraw, err error) {
	for rows.Next() {
		var withdraw types.Withdraw
		if err := rows.StructScan(&withdraw); err != nil {
			return withdraws, err
		}

		withdraws = append(withdraws, withdraw)
	}

	return withdraws, rows.Err()
}

func (s *WithdrawService) Insert(withdrawal types.Withdraw) error {
	sql := `INSERT INTO withdraws (exchange, asset, network, address, amount, txn_id, txn_fee, time)
			VALUES (:exchange, :asset, :network, :address, :amount, :txn_id, :txn_fee, :time)`
	_, err := s.DB.NamedExec(sql, withdrawal)
	return err
}
