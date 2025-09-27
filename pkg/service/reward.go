package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	exchange2 "github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// RewardService collects the reward records from the exchange,
// currently it's only available for MAX exchange.
// TODO: add summary query for calculating the reward amounts
// CREATE VIEW reward_summary_by_years AS SELECT YEAR(created_at) as year, reward_type, currency, SUM(quantity) FROM rewards WHERE reward_type != 'airdrop' GROUP BY YEAR(created_at), reward_type, currency ORDER BY year DESC;
type RewardService struct {
	DB      *sqlx.DB
	dialect DatabaseDialect
}

func NewRewardService(db *sqlx.DB) *RewardService {
	return &RewardService{
		DB:      db,
		dialect: GetDialect(db.DriverName()),
	}
}

func (s *RewardService) ensureDialect() DatabaseDialect {
	if s.dialect == nil {
		s.dialect = GetDialect(s.DB.DriverName())
	}
	return s.dialect
}

func (s *RewardService) Sync(ctx context.Context, exchange types.Exchange, startTime time.Time) error {
	api, ok := exchange.(types.ExchangeRewardService)
	if !ok {
		return ErrExchangeRewardServiceNotImplemented
	}

	isMargin, isFutures, _, _ := exchange2.GetSessionAttributes(exchange)
	if isMargin || isFutures {
		return nil
	}

	tasks := []SyncTask{
		{
			Type:   types.Reward{},
			Select: SelectLastRewards(exchange.Name(), 100),
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.RewardBatchQuery{
					Service: api,
				}
				return query.Query(ctx, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.Reward).CreatedAt.Time()
			},
			ID: func(obj interface{}) string {
				reward := obj.(types.Reward)
				return string(reward.Type) + "_" + reward.UUID
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

type CurrencyPositionMap map[string]fixedpoint.Value

func (s *RewardService) AggregateUnspentCurrencyPosition(ctx context.Context, ex types.ExchangeName, since time.Time) (CurrencyPositionMap, error) {
	m := make(CurrencyPositionMap)

	rewards, err := s.QueryUnspentSince(ctx, ex, since)
	if err != nil {
		return nil, err
	}

	for _, reward := range rewards {
		m[reward.Currency] = m[reward.Currency].Add(reward.Quantity)
	}

	return m, nil
}

func (s *RewardService) QueryUnspentSince(ctx context.Context, ex types.ExchangeName, since time.Time, rewardTypes ...types.RewardType) ([]types.Reward, error) {
	d := s.ensureDialect()
	sql := "SELECT * FROM rewards WHERE created_at >= :since AND exchange = :exchange AND spent IS FALSE "

	args := map[string]interface{}{
		"exchange": ex,
		"since":    since,
	}

	if len(rewardTypes) == 0 {
		sql += " AND " + d.EscapeColumnName("reward_type") + " NOT IN ('airdrop') "
	} else {
		var placeholders []string
		for i, n := range rewardTypes {
			key := fmt.Sprintf("rt%d", i)
			placeholders = append(placeholders, ":"+key)
			args[key] = string(n)
		}
		sql += " AND " + d.EscapeColumnName("reward_type") + " IN (" + strings.Join(placeholders, ", ") + ") "
	}

	sql += " ORDER BY created_at ASC"

	rows, err := s.DB.NamedQueryContext(ctx, sql, args)

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return s.scanRows(rows)
}

func (s *RewardService) QueryUnspent(ctx context.Context, ex types.ExchangeName, rewardTypes ...types.RewardType) ([]types.Reward, error) {
	d := s.ensureDialect()
	sql := "SELECT * FROM rewards WHERE exchange = :exchange AND spent IS FALSE "
	args := map[string]interface{}{
		"exchange": ex,
	}
	if len(rewardTypes) == 0 {
		sql += " AND " + d.EscapeColumnName("reward_type") + " NOT IN ('airdrop') "
	} else {
		var placeholders []string
		for i, n := range rewardTypes {
			key := fmt.Sprintf("rt%d", i)
			placeholders = append(placeholders, ":"+key)
			args[key] = string(n)
		}
		sql += " AND " + d.EscapeColumnName("reward_type") + " IN (" + strings.Join(placeholders, ", ") + ") "
	}

	sql += " ORDER BY created_at ASC"
	rows, err := s.DB.NamedQueryContext(ctx, sql, args)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return s.scanRows(rows)
}

func (s *RewardService) MarkCurrencyAsSpent(ctx context.Context, currency string) error {
	d := s.ensureDialect()
	result, err := s.DB.NamedExecContext(ctx, "UPDATE "+d.EscapeTableName("rewards")+" SET "+d.EscapeColumnName("spent")+" = TRUE WHERE "+d.EscapeColumnName("currency")+" = :currency AND "+d.EscapeColumnName("spent")+" IS FALSE", map[string]interface{}{
		"currency": currency,
	})

	if err != nil {
		return err
	}

	_, err = result.RowsAffected()
	return err
}

func (s *RewardService) MarkAsSpent(ctx context.Context, uuid string) error {
	d := s.ensureDialect()
	result, err := s.DB.NamedExecContext(ctx, "UPDATE "+d.EscapeTableName("rewards")+" SET "+d.EscapeColumnName("spent")+" = TRUE WHERE "+d.EscapeColumnName("uuid")+" = :uuid", map[string]interface{}{
		"uuid": uuid,
	})
	if err != nil {
		return err
	}

	cnt, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if cnt == 0 {
		return fmt.Errorf("reward uuid:%s not found", uuid)
	}

	return nil
}

func (s *RewardService) scanRows(rows *sqlx.Rows) (rewards []types.Reward, err error) {
	for rows.Next() {
		var reward types.Reward
		if err := rows.StructScan(&reward); err != nil {
			return rewards, err
		}

		rewards = append(rewards, reward)
	}

	return rewards, rows.Err()
}

func (s *RewardService) Insert(reward types.Reward) error {
	_, err := s.DB.NamedExec(`
			INSERT INTO rewards (exchange, uuid, reward_type, currency, quantity, state, note, created_at)
			VALUES (:exchange, :uuid, :reward_type, :currency, :quantity, :state, :note, :created_at)`,
		reward)
	return err
}

func SelectLastRewards(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("rewards").
		Where(sq.And{
			sq.Eq{"exchange": ex},
		}).
		OrderBy("created_at DESC").
		Limit(limit)
}
