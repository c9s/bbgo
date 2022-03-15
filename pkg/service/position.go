package service

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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

func (s *PositionService) scanRows(rows *sqlx.Rows) (positions []types.Position, err error) {
	for rows.Next() {
		var p types.Position
		if err := rows.StructScan(&p); err != nil {
			return positions, err
		}

		positions = append(positions, p)
	}

	return positions, rows.Err()
}

func (s *PositionService) Insert(position *types.Position, trade types.Trade, profit fixedpoint.Value) error {
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
			"trade_id":             trade.ID,
			"exchange":             trade.Exchange,
			"side":                 trade.Side,
			"traded_at":            trade.Time,
		})
	return err
}
