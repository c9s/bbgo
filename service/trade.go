package service

import (
	"github.com/c9s/bbgo/pkg/bbgo/types"
	"github.com/jmoiron/sqlx"
)

type TradeService struct {
	db *sqlx.DB
}

func NewTradeService(db *sqlx.DB)*TradeService {
	return &TradeService{db}
}

// QueryLast queries the last trade from the database
func (s *TradeService) QueryLast(symbol string) (trade types.Trade, err error) {
	rows, err := s.db.NamedQuery(`SELECT * FROM trades WHERE symbol = :symbol ORDER BY gid DESC LIMIT 1`, map[string]interface{}{
		"symbol": symbol,
	})
	if err != nil {
		return trade, err
	}

	defer rows.Close()

	rows.Next()
	err = rows.StructScan(&trade)
	return trade, err

}

func (s *TradeService) Query(symbol string) (trades []types.Trade, err error) {
	rows, err := s.db.NamedQuery(`SELECT * FROM trades WHERE symbol = :symbol ORDER BY gid ASC`, map[string]interface{}{
		"symbol": symbol,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var trade types.Trade
		if err := rows.StructScan(&trade); err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}

	return trades, err
}

func (s *TradeService) Insert(trade types.Trade) error {
	_, err := s.db.NamedExec(`
			INSERT INTO trades (id, exchange, symbol, price, quantity, quote_quantity, side, is_buyer, is_maker, fee, fee_currency, traded_at)
			VALUES (:id, :exchange, :symbol, :price, :quantity, :quote_quantity, :side, :is_buyer, :is_maker, :fee, :fee_currency, :traded_at)`,
		trade)
	return err
}
