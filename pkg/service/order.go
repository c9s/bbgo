package service

import (
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

type OrderService struct {
	DB *sqlx.DB
}

func NewOrderService(db *sqlx.DB) *OrderService {
	return &OrderService{db}
}

// QueryLast queries the last order from the database
func (s *OrderService) QueryLast(ex types.ExchangeName, symbol string) (*types.Order, error) {
	log.Infof("querying last order exchange = %s AND symbol = %s", ex, symbol)

	rows, err := s.DB.NamedQuery(`SELECT * FROM orders WHERE exchange = :exchange AND symbol = :symbol ORDER BY gid DESC LIMIT 1`, map[string]interface{}{
		"exchange": ex,
		"symbol":   symbol,
	})

	if err != nil {
		return nil, errors.Wrap(err, "query last order error")
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	defer rows.Close()

	if rows.Next() {
		var order types.Order
		err = rows.StructScan(&order)
		return &order, err
	}

	return nil, rows.Err()
}

func (s *OrderService) Query(ex types.ExchangeName, symbol string) ([]types.Order, error) {
	rows, err := s.DB.NamedQuery(`SELECT * FROM orders WHERE exchange = :exchange AND symbol = :symbol ORDER BY gid ASC`, map[string]interface{}{
		"exchange": ex,
		"symbol":   symbol,
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
			INSERT INTO orders (exchange, order_id, client_order_id, order_type, status, symbol, price, stop_price, quantity, executed_quantity, side, is_working, time_in_force, created_at)
			VALUES (:exchange, :order_id, :client_order_id, :order_type, :status, :symbol, :price, :stop_price, :quantity, :executed_quantity, :side, :is_working, :time_in_force, :created_at)`,
		order)
	return err
}
