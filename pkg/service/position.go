package service

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	sq "github.com/Masterminds/squirrel"
)

type PositionService struct {
	DB *sqlx.DB
}

func NewPositionService(db *sqlx.DB) *PositionService {
	return &PositionService{db}
}

func (s *PositionService) Load(ctx context.Context, id int64) (*types.Position, error) {
	var pos types.Position

	rows, err := s.DB.NamedQuery("SELECT * FROM positions WHERE id = :id", map[string]interface{}{
		"id": id,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		err = rows.StructScan(&pos)
		return &pos, err
	}

	return nil, errors.Wrapf(ErrTradeNotFound, "position id:%d not found", id)
}

func (s *PositionService) Insert(
	position *types.Position,
	trade types.Trade,
	profit, netProfit fixedpoint.Value,
) error {
	_, err := s.DB.NamedExec(`
		INSERT INTO positions (
			strategy,
			strategy_instance_id,
			symbol,
			quote_currency,
			base_currency,
			average_cost,
		   	base,
		    quote,
			profit,
			net_profit,
			trade_id,
		    exchange,
		    side,
			traded_at
		) VALUES (
			:strategy,
			:strategy_instance_id,
			:symbol,
			:quote_currency,
			:base_currency,
			:average_cost,
		    :base,
		    :quote,
			:profit,
			:net_profit,
			:trade_id,
		    :exchange,
		    :side,
			:traded_at
	    )`,
		map[string]interface{}{
			"strategy":             position.Strategy,
			"strategy_instance_id": position.StrategyInstanceID,
			"symbol":               position.Symbol,
			"quote_currency":       position.QuoteCurrency,
			"base_currency":        position.BaseCurrency,
			"average_cost":         position.AverageCost,
			"base":                 position.Base,
			"quote":                position.Quote,
			"profit":               profit,
			"net_profit":           netProfit,
			"trade_id":             trade.ID,
			"exchange":             trade.Exchange,
			"side":                 trade.Side,
			"traded_at":            trade.Time,
		})
	return err
}

type PositionQueryOptions struct {
	Strategy           string
	StrategyInstanceID string
	Symbol             string
	StartTime          time.Time // inclusive
	EndTime            time.Time // inclusive
}

func (s *PositionService) Delete(options PositionQueryOptions) error {
	del := sq.Delete("positions")
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
	_, err = s.DB.Exec(sql, args...)
	return err
}
