package service

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/types"
)

type ProfitService struct {
	DB *sqlx.DB
}

func NewProfitService(db *sqlx.DB) *ProfitService {
	return &ProfitService{db}
}

func (s *ProfitService) Load(ctx context.Context, id int64) (*types.Trade, error) {
	var trade types.Trade

	rows, err := s.DB.NamedQuery("SELECT * FROM trades WHERE id = :id", map[string]interface{}{
		"id": id,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		err = rows.StructScan(&trade)
		return &trade, err
	}

	return nil, errors.Wrapf(ErrTradeNotFound, "trade id:%d not found", id)
}

func (s *ProfitService) scanRows(rows *sqlx.Rows) (trades []types.Trade, err error) {
	for rows.Next() {
		var trade types.Trade
		if err := rows.StructScan(&trade); err != nil {
			return trades, err
		}

		trades = append(trades, trade)
	}

	return trades, rows.Err()
}

func (s *ProfitService) Insert(trade types.Trade) error {
	_, err := s.DB.NamedExec(`
			INSERT INTO profits (id, exchange, symbol, trade_id, average_cost, profit, price, quantity, quote_quantity, side, traded_at, is_margin, is_futures, is_isolated)
			VALUES (:id, :exchange, :order_id, :symbol, :price, :quantity, :quote_quantity, :side, :is_buyer, :is_maker, :fee, :fee_currency, :traded_at, :is_margin, :is_futures, :is_isolated)`,
		trade)
	return err
}
