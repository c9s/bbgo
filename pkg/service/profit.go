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

func (s *ProfitService) scanRows(rows *sqlx.Rows) (profits []types.Profit, err error) {
	for rows.Next() {
		var profit types.Profit
		if err := rows.StructScan(&profit); err != nil {
			return profits, err
		}

		profits = append(profits, profit)
	}

	return profits, rows.Err()
}

func (s *ProfitService) Insert(profit types.Profit) error {
	_, err := s.DB.NamedExec(`
		INSERT INTO profits (
			strategy,
			strategy_instance_id,
			symbol,
			quote_currency,
			base_currency,
			average_cost,
			profit,
			net_profit,
			profit_margin,
			net_profit_margin,
			trade_id,
			price,
			quantity,
			quote_quantity,
			side,
			is_buyer,
			is_maker,
			fee,
			fee_currency,
			fee_in_usd,
			traded_at,
			exchange,
			is_margin,
			is_futures,
			is_isolated
		) VALUES (
			:strategy,
			:strategy_instance_id,
			:symbol,
			:quote_currency,
			:base_currency,
			:average_cost,
			:profit,
			:net_profit,
			:profit_margin,
			:net_profit_margin,
			:trade_id,
			:price,
			:quantity,
			:quote_quantity,
			:side,
			:is_buyer,
			:is_maker,
			:fee,
			:fee_currency,
			:fee_in_usd,
			:traded_at,
			:exchange,
			:is_margin,
			:is_futures,
			:is_isolated
	    )`,
		profit)
	return err
}
