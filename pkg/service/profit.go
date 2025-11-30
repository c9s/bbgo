package service

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/types"
)

type ProfitService struct {
	DB *sqlx.DB
}

func (s *ProfitService) Load(ctx context.Context, id int64) (*types.Trade, error) {
	var trade types.Trade

	rows, err := s.DB.NamedQueryContext(ctx, "SELECT * FROM trades WHERE id = :id", map[string]interface{}{
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := s.DB.NamedExecContext(ctx, `
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

type ProfitQueryOptions struct {
	Strategy           string
	StrategyInstanceID string
	Symbol             string
	StartTime          time.Time // inclusive
	EndTime            time.Time // inclusive
}

func (s *ProfitService) Delete(ctx context.Context, options ProfitQueryOptions) error {
	del := sq.Delete("profits")
	if options.Strategy != "" {
		del = del.Where(sq.Eq{"strategy": options.Strategy})
	}
	if options.StrategyInstanceID != "" {
		del = del.Where(sq.Eq{"strategy_instance_id": options.StrategyInstanceID})
	}
	if options.Symbol != "" {
		del = del.Where(sq.Eq{"symbol": options.Symbol})
	}
	if !options.StartTime.IsZero() {
		del = del.Where(sq.GtOrEq{"traded_at": options.StartTime})
	}
	if !options.EndTime.IsZero() {
		del = del.Where(sq.LtOrEq{"traded_at": options.EndTime})
	}
	sql, args, err := del.ToSql()
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, sql, args...)
	return err
}
