package service

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	exchange2 "github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

type OrderService struct {
    DB      *sqlx.DB
    dialect DatabaseDialect
}

func NewOrderService(db *sqlx.DB) *OrderService {
    return &OrderService{
        DB:      db,
        dialect: GetDialect(db.DriverName()),
    }
}

func (s *OrderService) ensureDialect() DatabaseDialect {
    if s.dialect == nil {
        s.dialect = GetDialect(s.DB.DriverName())
    }
    return s.dialect
}

func (s *OrderService) Sync(
	ctx context.Context, exchange types.Exchange, symbol string,
	startTime, endTime time.Time,
) error {
	isMargin, isFutures, isIsolated, isolatedSymbol := exchange2.GetSessionAttributes(exchange)
	// override symbol if isolatedSymbol is not empty
	if isIsolated && len(isolatedSymbol) > 0 {
		symbol = isolatedSymbol
	}
	logger := util.GetLoggerFromCtx(ctx)
	logger.Infof("session attributes: isMargin=%v isFutures=%v isIsolated=%v isolatedSymbol=%s", isMargin, isFutures, isIsolated, isolatedSymbol)

	api, ok := exchange.(types.ExchangeTradeHistoryService)
	if !ok {
		logger.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing orders", exchange.Name())
		return nil
	}

	lastOrderID := uint64(0)
	tasks := []SyncTask{
		{
			Type: types.Order{},
			Time: func(obj interface{}) time.Time {
				return obj.(types.Order).CreationTime.Time()
			},
			ID: func(obj interface{}) string {
				order := obj.(types.Order)
				return strconv.FormatUint(order.OrderID, 10)
			},
			Select: SelectLastOrders(exchange.Name(), symbol, isMargin, isFutures, isIsolated, 100),
			OnLoad: func(objs interface{}) {
				// update last order ID
				orders := objs.([]types.Order)
				if len(orders) > 0 {
					end := len(orders) - 1
					last := orders[end]
					lastOrderID = last.OrderID
				}
			},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.ClosedOrderBatchQuery{
					ExchangeTradeHistoryService: api,
				}

				return query.Query(ctx, symbol, startTime, endTime, lastOrderID)
			},
			Filter: func(obj interface{}) bool {
				// skip canceled and not filled orders
				order := obj.(types.Order)
				if order.Status == types.OrderStatusCanceled && order.ExecutedQuantity.IsZero() {
					return false
				}

				return true
			},
			Insert: func(obj interface{}) error {
				order := obj.(types.Order)
				return s.Insert(order)
			},
			LogInsert: true,
		},
	}

	for _, sel := range tasks {
		if err := sel.execute(ctx, s.DB, startTime, endTime); err != nil {
			return err
		}
	}

	return nil
}

func SelectLastOrders(ex types.ExchangeName, symbol string, isMargin, isFutures, isIsolated bool, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("orders").
		Where(sq.And{
			sq.Eq{"symbol": symbol},
			sq.Eq{"exchange": ex},
			sq.Eq{"is_margin": isMargin},
			sq.Eq{"is_futures": isFutures},
			sq.Eq{"is_isolated": isIsolated},
		}).
		OrderBy("created_at DESC").
		Limit(limit)
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
	sql := genOrderSQL(s.DB.DriverName(), options)

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

func genOrderSQL(driver string, options QueryOrdersOptions) string {
	dialect := GetDialect(driver)

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
			where = append(where, "orders.gid > :gid")
		case "DESC":
			where = append(where, "orders.gid < :gid")

		}
	}

	if len(options.Exchange) > 0 {
		where = append(where, "orders.exchange = :exchange")
	}
	if len(options.Symbol) > 0 {
		where = append(where, "orders.symbol = :symbol")
	}

	// Build column list. In MySQL, expand columns to apply BIN_TO_UUID on uuid.
	var selColumns []string
	if driver == "mysql" {
		to := reflect.TypeOf(types.Order{})
		for i := 0; i < to.NumField(); i++ {
			field := to.Field(i)
			colName := field.Tag.Get("db")
			if colName == "" || colName == "-" {
				continue
			}
			if colName == "uuid" {
				selColumns = append(selColumns, dialectUuidSelector(dialect, "orders", "uuid"))
			} else {
				selColumns = append(selColumns, "orders."+colName)
			}
		}
	} else {
		selColumns = append(selColumns, "orders.*")
	}

	// Compute average price via an aggregated subquery to avoid GROUP BY issues on Postgres
	// and to keep outer select free of aggregates.
	avgSub := `SELECT order_id, exchange, SUM(price * quantity)/NULLIF(SUM(quantity), 0) AS avg_price FROM trades GROUP BY exchange, order_id`
	// Cross-database null-coalesce for fallback to orders.price when no trades
	avgColumn := dialect.Coalesce("avg_trades.avg_price", "orders.price") + " AS average_price"

	sql := `SELECT ` + strings.Join(selColumns, ", ") + `, ` + avgColumn + ` FROM orders` +
		` LEFT JOIN (` + avgSub + `) AS avg_trades ON (avg_trades.order_id = orders.order_id AND avg_trades.exchange = orders.exchange)`
	if len(where) > 0 {
		sql += ` WHERE ` + strings.Join(where, " AND ")
	}
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
    d := s.ensureDialect()
    insertClause := "exchange, order_id, client_order_id, order_type, status, symbol, price, stop_price, quantity, executed_quantity, side, is_working, time_in_force, created_at, updated_at, is_margin, is_futures, is_isolated, uuid"

    // Use dialect-aware UUID handling
    valuesClause := ":exchange, :order_id, :client_order_id, :order_type, :status, :symbol, :price, :stop_price, :quantity, :executed_quantity, :side, :is_working, :time_in_force, :created_at, :updated_at, :is_margin, :is_futures, :is_isolated, " + d.IfExpr(":uuid != ''", d.UUIDBinaryConversion(":uuid"), "''")

    updateClause := "status=:status, executed_quantity=:executed_quantity, is_working=:is_working, updated_at=:updated_at"

    sql := d.OrderUpsertSQL(insertClause, valuesClause, updateClause)
    _, err = s.DB.NamedExec(sql, order)
    return err
}
