package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

type OrderService struct {
	DB *sqlx.DB
}

func (s *OrderService) Sync(ctx context.Context, exchange types.Exchange, symbol string, startTime time.Time) error {
	isMargin := false
	isFutures := false
	isIsolated := false

	if marginExchange, ok := exchange.(types.MarginExchange); ok {
		marginSettings := marginExchange.GetMarginSettings()
		isMargin = marginSettings.IsMargin
		isIsolated = marginSettings.IsIsolatedMargin
		if marginSettings.IsIsolatedMargin {
			symbol = marginSettings.IsolatedMarginSymbol
		}
	}

	if futuresExchange, ok := exchange.(types.FuturesExchange); ok {
		futuresSettings := futuresExchange.GetFuturesSettings()
		isFutures = futuresSettings.IsFutures
		isIsolated = futuresSettings.IsIsolatedFutures
		if futuresSettings.IsIsolatedFutures {
			symbol = futuresSettings.IsolatedFuturesSymbol
		}
	}

	records, err := s.QueryLast(exchange.Name(), symbol, isMargin, isFutures, isIsolated, 50)
	if err != nil {
		return err
	}

	orderKeys := make(map[uint64]struct{})

	var lastID uint64 = 0
	if len(records) > 0 {
		for _, record := range records {
			orderKeys[record.OrderID] = struct{}{}
		}

		lastID = records[0].OrderID
		startTime = records[0].CreationTime.Time()
	}

	b := &batch.ClosedOrderBatchQuery{Exchange: exchange}
	ordersC, errC := b.Query(ctx, symbol, startTime, time.Now(), lastID)
	for order := range ordersC {
		select {

		case <-ctx.Done():
			return ctx.Err()

		case err := <-errC:
			if err != nil {
				return err
			}

		default:

		}

		if _, exists := orderKeys[order.OrderID]; exists {
			continue
		}

		// skip canceled and not filled orders
		if order.Status == types.OrderStatusCanceled && order.ExecutedQuantity.IsZero() {
			continue
		}

		if err := s.Insert(order); err != nil {
			return err
		}
	}

	return <-errC
}


// QueryLast queries the last order from the database
func (s *OrderService) QueryLast(ex types.ExchangeName, symbol string, isMargin, isFutures, isIsolated bool, limit int) ([]types.Order, error) {
	log.Infof("querying last order exchange = %s AND symbol = %s AND is_margin = %v AND is_futures = %v AND is_isolated = %v", ex, symbol, isMargin, isFutures, isIsolated)

	sql := `SELECT * FROM orders WHERE exchange = :exchange AND symbol = :symbol AND is_margin = :is_margin AND is_futures = :is_futures AND is_isolated = :is_isolated ORDER BY gid DESC LIMIT :limit`
	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"exchange":    ex,
		"symbol":      symbol,
		"is_margin":   isMargin,
		"is_futures":  isFutures,
		"is_isolated": isIsolated,
		"limit":       limit,
	})

	if err != nil {
		return nil, errors.Wrap(err, "query last order error")
	}

	defer rows.Close()
	return s.scanRows(rows)
}

type AggOrder struct {
	types.Order
	AveragePrice *float64 `json:"averagePrice" db:"average_price"`
}

type QueryOrdersOptions struct {
	Exchange types.ExchangeName
	Symbol   string
	LastGID  int64
	Ordering string
}

func (s *OrderService) Query(options QueryOrdersOptions) ([]AggOrder, error) {
	sql := genOrderSQL(options)

	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"exchange": options.Exchange,
		"symbol":   options.Symbol,
		"gid":      options.LastGID,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanAggRows(rows)
}

func genOrderSQL(options QueryOrdersOptions) string {
	// ascending
	ordering := "ASC"
	switch v := strings.ToUpper(options.Ordering); v {
	case "DESC", "ASC":
		ordering = options.Ordering
	}

	var where []string
	if options.LastGID > 0 {
		switch ordering {
		case "ASC":
			where = append(where, "gid > :gid")
		case "DESC":
			where = append(where, "gid < :gid")

		}
	}

	if len(options.Exchange) > 0 {
		where = append(where, "exchange = :exchange")
	}
	if len(options.Symbol) > 0 {
		where = append(where, "symbol = :symbol")
	}

	sql := `SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders` +
		` LEFT JOIN trades AS t ON (t.order_id = orders.order_id)`
	if len(where) > 0 {
		sql += ` WHERE ` + strings.Join(where, " AND ")
	}
	sql += ` GROUP BY orders.gid `
	sql += ` ORDER BY orders.gid ` + ordering
	sql += ` LIMIT ` + strconv.Itoa(500)

	log.Info(sql)
	return sql
}

func (s *OrderService) scanAggRows(rows *sqlx.Rows) (orders []AggOrder, err error) {
	for rows.Next() {
		var order AggOrder
		if err := rows.StructScan(&order); err != nil {
			return nil, err
		}

		orders = append(orders, order)
	}

	return orders, rows.Err()
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

func (s *OrderService) Insert(order types.Order) (err error) {
	if s.DB.DriverName() == "mysql" {
		_, err = s.DB.NamedExec(`
			INSERT INTO orders (exchange, order_id, client_order_id, order_type, status, symbol, price, stop_price, quantity, executed_quantity, side, is_working, time_in_force, created_at, updated_at, is_margin, is_futures, is_isolated)
			VALUES (:exchange, :order_id, :client_order_id, :order_type, :status, :symbol, :price, :stop_price, :quantity, :executed_quantity, :side, :is_working, :time_in_force, :created_at, :updated_at, :is_margin, :is_futures, :is_isolated)
			ON DUPLICATE KEY UPDATE status=:status, executed_quantity=:executed_quantity, is_working=:is_working, updated_at=:updated_at`, order)
		return err
	}

	_, err = s.DB.NamedExec(`
			INSERT INTO orders (exchange, order_id, client_order_id, order_type, status, symbol, price, stop_price, quantity, executed_quantity, side, is_working, time_in_force, created_at, updated_at, is_margin, is_futures, is_isolated)
			VALUES (:exchange, :order_id, :client_order_id, :order_type, :status, :symbol, :price, :stop_price, :quantity, :executed_quantity, :side, :is_working, :time_in_force, :created_at, :updated_at, :is_margin, :is_futures, :is_isolated)
	`, order)

	return err
}
