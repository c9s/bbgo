package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// RewardService collects the reward records from the exchange,
// currently it's only available for MAX exchange.
// TODO: add summary query for calculating the reward amounts
// CREATE VIEW reward_summary_by_years AS SELECT YEAR(created_at) as year, reward_type, currency, SUM(quantity) FROM rewards WHERE reward_type != 'airdrop' GROUP BY YEAR(created_at), reward_type, currency ORDER BY year DESC;
type RewardService struct {
	DB *sqlx.DB
}

func (s *RewardService) QueryLast(ex types.ExchangeName, limit int) ([]types.Reward, error) {
	sql := "SELECT * FROM `rewards` WHERE `exchange` = :exchange ORDER BY `created_at` DESC LIMIT :limit"
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

func (s *RewardService) Sync(ctx context.Context, exchange types.Exchange) error {
	service, ok := exchange.(types.ExchangeRewardService)
	if !ok {
		return ErrExchangeRewardServiceNotImplemented
	}

	var rewardKeys = map[string]struct{}{}

	var startTime time.Time

	records, err := s.QueryLast(exchange.Name(), 50)
	if err != nil {
		return err
	}

	if len(records) > 0 {
		lastRecord := records[0]
		startTime = lastRecord.CreatedAt.Time()

		for _, record := range records {
			rewardKeys[record.UUID] = struct{}{}
		}
	}

	batchQuery := &batch.RewardBatchQuery{Service: service}
	rewardsC, errC := batchQuery.Query(ctx, startTime, time.Now())

	for reward := range rewardsC {
		select {

		case <-ctx.Done():
			return ctx.Err()

		case err := <-errC:
			if err != nil {
				return err
			}

		default:

		}

		if _, ok := rewardKeys[reward.UUID]; ok {
			continue
		}

		logrus.Infof("inserting reward: %s %s %s %f %s", reward.Exchange, reward.Type, reward.Currency, reward.Quantity.Float64(), reward.CreatedAt)

		if err := s.Insert(reward); err != nil {
			return err
		}
	}

	return <-errC
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
	sql := "SELECT * FROM rewards WHERE created_at >= :since AND exchange = :exchange AND spent IS FALSE "

	if len(rewardTypes) == 0 {
		sql += " AND `reward_type` NOT IN ('airdrop') "
	} else {
		var args []string
		for _, n := range rewardTypes {
			args = append(args, strconv.Quote(string(n)))
		}
		sql += " AND `reward_type` IN (" + strings.Join(args, ", ") + ") "
	}

	sql += " ORDER BY created_at ASC"

	rows, err := s.DB.NamedQueryContext(ctx, sql, map[string]interface{}{
		"exchange":   ex,
		"since": since,
	})

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return s.scanRows(rows)
}

func (s *RewardService) QueryUnspent(ctx context.Context, ex types.ExchangeName, rewardTypes ...types.RewardType) ([]types.Reward, error) {
	sql := "SELECT * FROM rewards WHERE exchange = :exchange AND spent IS FALSE "
	if len(rewardTypes) == 0 {
		sql += " AND `reward_type` NOT IN ('airdrop') "
	} else {
		var args []string
		for _, n := range rewardTypes {
			args = append(args, strconv.Quote(string(n)))
		}
		sql += " AND `reward_type` IN (" + strings.Join(args, ", ") + ") "
	}

	sql += " ORDER BY created_at ASC"
	rows, err := s.DB.NamedQueryContext(ctx, sql, map[string]interface{}{
		"exchange": ex,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return s.scanRows(rows)
}

func (s *RewardService) MarkCurrencyAsSpent(ctx context.Context, currency string) error {
	result, err := s.DB.NamedExecContext(ctx, "UPDATE `rewards` SET `spent` = TRUE WHERE `currency` = :currency AND `spent` IS FALSE", map[string]interface{}{
		"currency": currency,
	})

	if err != nil {
		return err
	}

	_, err = result.RowsAffected()
	return err
}

func (s *RewardService) MarkAsSpent(ctx context.Context, uuid string) error {
	result, err := s.DB.NamedExecContext(ctx, "UPDATE `rewards` SET `spent` = TRUE WHERE `uuid` = :uuid", map[string]interface{}{
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
