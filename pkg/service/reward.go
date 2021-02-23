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
func (s *RewardService) Query(ex types.ExchangeName, rewardType string, from time.Time) ([]types.Trade, error) {
	rows, err := s.DB.NamedQuery(`SELECT * FROM trades WHERE exchange = :exchange AND (symbol = :symbol OR fee_currency = :fee_currency) ORDER BY traded_at ASC`, map[string]interface{}{
		"exchange":    ex,
		"reward_type": rewardType,
		"from": from.Unix(),
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
}

func (s *RewardService) scanRows(rows *sqlx.Rows) (trades []types.Trade, err error) {
	for rows.Next() {
		var trade types.Trade
		if err := rows.StructScan(&trade); err != nil {
			return trades, err
		}

		trades = append(trades, trade)
	}

	return trades, rows.Err()
}

func (s *RewardService) Insert(trade types.Trade) error {
	_, err := s.DB.NamedExec(`
			INSERT INTO trades (id, exchange, order_id, symbol, price, quantity, quote_quantity, side, is_buyer, is_maker, fee, fee_currency, traded_at, is_margin, is_isolated)
			VALUES (:id, :exchange, :order_id, :symbol, :price, :quantity, :quote_quantity, :side, :is_buyer, :is_maker, :fee, :fee_currency, :traded_at, :is_margin, :is_isolated)`,
		trade)
	return err
}

