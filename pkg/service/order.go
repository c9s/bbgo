package service

import (
	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/types"
)

type OrderService struct {
	DB *sqlx.DB
}

func NewOrderService(db *sqlx.DB) *OrderService {
	return &OrderService{db}
}

func (s *OrderService) Query(symbol string) ([]types.Order, error) {
	rows, err := s.DB.NamedQuery(`SELECT * FROM orders WHERE symbol = :symbol ORDER BY gid ASC`, map[string]interface{}{
		"symbol": symbol,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
}

func (s *OrderService) scanRows(rows *sqlx.Rows) (orders []types.Order, err error) {
	for rows.Next() {
		var order types.Order
		if err := rows.StructScan(&order); err != nil {
			return nil, err
		}

		orders = append(orders, order)
	}

	return orders, rows.Err()
}

func (s *OrderService) Insert(order types.Order) error {
	_, err := s.DB.NamedExec(`
			INSERT INTO orders (id, exchange, symbol, price, quantity, quote_quantity, side, fee, fee_currency, traded_at)
			VALUES (:id, :exchange, :symbol, :price, :quantity, :quote_quantity, :side, :fee, :fee_currency, :traded_at)`,
		order)
	return err
}
