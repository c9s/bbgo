package service

import (
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/types"
)

type RewardService struct {
	DB *sqlx.DB
}

func NewRewardService(db *sqlx.DB) *RewardService {
	return &RewardService{db}
}

func (s *RewardService) QueryLast(ex types.ExchangeName, limit int) ([]types.Reward, error) {
	rows, err := s.DB.NamedQuery(`SELECT * FROM rewards WHERE exchange = :exchange ORDER BY created_at DESC LIMIT :limit`, map[string]interface{}{
		"exchange": ex,
		"limit":    limit,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return s.scanRows(rows)
}

func (s *RewardService) QueryUnspent(ex types.ExchangeName, from time.Time) ([]types.Reward, error) {
	rows, err := s.DB.NamedQuery(`SELECT * FROM rewards WHERE exchange = :exchange AND spent IS FALSE ORDER BY created_at ASC`, map[string]interface{}{
		"exchange": ex,
		"from":     from,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
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
			INSERT INTO rewards (exchange, uuid, reward_type, quantity, state, created_at)
			VALUES (:exchange, :uuid, :reward_type, :quantity, :state, :created_at)`,
		reward)
	return err
}
