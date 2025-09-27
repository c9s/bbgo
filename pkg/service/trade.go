package service

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	exchange2 "github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var ErrTradeNotFound = errors.New("trade not found")

type QueryTradesOptions struct {
	Exchange types.ExchangeName
	Sessions []string
	Symbol   string
	LastGID  int64

	// inclusive
	Since *time.Time

	// exclusive
	Until *time.Time

	// ASC or DESC
	Ordering string

	// OrderByColumn is the column name to order by
	// Currently we only support traded_at and gid column.
	OrderByColumn string
	Limit         uint64
}

type TradingVolume struct {
	Year        int       `db:"year" json:"year"`
	Month       int       `db:"month" json:"month,omitempty"`
	Day         int       `db:"day" json:"day,omitempty"`
	Time        time.Time `json:"time,omitempty"`
	Exchange    string    `db:"exchange" json:"exchange,omitempty"`
	Symbol      string    `db:"symbol" json:"symbol,omitempty"`
	QuoteVolume float64   `db:"quote_volume" json:"quoteVolume"`
}

type TradingVolumeQueryOptions struct {
	GroupByPeriod string
	SegmentBy     string
}

type TradeService struct {
    DB      *sqlx.DB
    dialect DatabaseDialect
}

func NewTradeService(db *sqlx.DB) *TradeService {
    return &TradeService{
        DB:      db,
        dialect: GetDialect(db.DriverName()),
    }
}

// ensureDialect initializes the dialect if the service was constructed without NewTradeService.
func (s *TradeService) ensureDialect() DatabaseDialect {
    if s.dialect == nil {
        s.dialect = GetDialect(s.DB.DriverName())
    }
    return s.dialect
}

func (s *TradeService) Sync(
	ctx context.Context,
	exchange types.Exchange, symbol string,
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
		logger.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing trades", exchange.Name())
		return nil
	}

	lastTradeID := uint64(1)
	tasks := []SyncTask{
		{
			Type:   types.Trade{},
			Select: SelectLastTrades(exchange.Name(), symbol, isMargin, isFutures, isIsolated, 100),
			OnLoad: func(objs interface{}) {
				// update last trade ID
				trades := objs.([]types.Trade)
				if len(trades) > 0 {
					end := len(trades) - 1
					last := trades[end]
					lastTradeID = last.ID
				}
				logger.Infof("on load: last trade ID: %d", lastTradeID)
			},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.TradeBatchQuery{
					ExchangeTradeHistoryService: api,
				}
				logger.Infof("sync trades from %s to %s, lastTradeID: %d", startTime, endTime, lastTradeID)
				return query.Query(ctx, symbol, &types.TradeQueryOptions{
					StartTime:   &startTime,
					EndTime:     &endTime,
					LastTradeID: lastTradeID,
				})
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.Trade).Time.Time()
			},
			ID: func(obj interface{}) string {
				trade := obj.(types.Trade)
				id := strconv.FormatUint(trade.ID, 10) + trade.Side.String()
				return id
			},
			Insert: func(obj interface{}) error {
				trade := obj.(types.Trade)
				return s.Insert(trade)
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

func (s *TradeService) QueryTradingVolume(startTime time.Time, options TradingVolumeQueryOptions) ([]TradingVolume, error) {
	args := map[string]interface{}{
		// "symbol":      symbol,
		// "exchange":    ex,
		// "is_margin":   isMargin,
		// "is_isolated": isIsolated,
		"start_time": startTime,
	}

	dialect := GetDialect(s.DB.DriverName())
	sql := GenerateTradingVolumeQuerySQL(dialect, options)

	log.Info(sql)

	rows, err := s.DB.NamedQuery(sql, args)
	if err != nil {
		return nil, errors.Wrap(err, "query last trade error")
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	defer rows.Close()

	var records []TradingVolume
	for rows.Next() {
		var record TradingVolume
		err = rows.StructScan(&record)
		if err != nil {
			return records, err
		}

		record.Time = time.Date(record.Year, time.Month(record.Month), record.Day, 0, 0, 0, 0, time.Local)
		records = append(records, record)
	}

	return records, rows.Err()
}

// GenerateTradingVolumeQuerySQL builds a cross-dialect SQL statement that aggregates trade
// records into quote currency volume (SUM(quantity * price)) since :start_time. The
// aggregation granularity is determined by options.GroupByPeriod ("day", "month", "year"),
// using dialect-specific date extraction functions to produce year/month/day selector,
// GROUP BY, and ORDER BY components. If options.SegmentBy is "symbol" or "exchange", the
// respective column is additionally selected and grouped. The query always filters on
// traded_at > :start_time and expects the caller to bind that named parameter (and to
// append any further constraints if needed). It returns the raw SQL string only; value
// binding and execution are the caller's responsibility.
func GenerateTradingVolumeQuerySQL(dialect DatabaseDialect, options TradingVolumeQueryOptions) string {
	timeRangeColumn := "traded_at"
	sel, groupBys, orderBys := GenerateTimeRangeClauses(dialect, timeRangeColumn, options.GroupByPeriod)

	switch options.SegmentBy {
	case "symbol":
		sel = append(sel, "symbol")
		groupBys = append([]string{"symbol"}, groupBys...)
		orderBys = append(orderBys, "symbol")
	case "exchange":
		sel = append(sel, "exchange")
		groupBys = append([]string{"exchange"}, groupBys...)
		orderBys = append(orderBys, "exchange")
	}

	sel = append(sel, "SUM(quantity * price) AS quote_volume")
	where := []string{timeRangeColumn + " > :start_time"}
	sql := `SELECT ` + strings.Join(sel, ", ") + ` FROM trades` +
		` WHERE ` + strings.Join(where, " AND ") +
		` GROUP BY ` + strings.Join(groupBys, ", ") +
		` ORDER BY ` + strings.Join(orderBys, ", ")

	return sql
}

func (s *TradeService) QueryForTradingFeeCurrency(ex types.ExchangeName, symbol string, feeCurrency string) ([]types.Trade, error) {
	sql := "SELECT " + strings.Join(genTradeSelectColumns(s.DB.DriverName()), ", ") + " FROM trades WHERE exchange = :exchange AND (symbol = :symbol OR fee_currency = :fee_currency) ORDER BY traded_at ASC"
	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"exchange":     ex,
		"symbol":       symbol,
		"fee_currency": feeCurrency,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
}

func (s *TradeService) Query(options QueryTradesOptions) ([]types.Trade, error) {
	sel := sq.Select(genTradeSelectColumns(s.DB.DriverName())...).
		From("trades")

	if options.LastGID != 0 {
		sel = sel.Where(sq.Gt{"gid": options.LastGID})
	}
	if options.Since != nil {
		sel = sel.Where(sq.GtOrEq{"traded_at": options.Since})
	}
	if options.Until != nil {
		sel = sel.Where(sq.Lt{"traded_at": options.Until})
	}

	if options.Symbol != "" {
		sel = sel.Where(sq.Eq{"symbol": options.Symbol})
	}

	if options.Exchange != "" {
		sel = sel.Where(sq.Eq{"exchange": options.Exchange})
	}

	if len(options.Sessions) > 0 {
		// FIXME: right now we only have the exchange field in the db, we might need to add the session field too.
		sel = sel.Where(sq.Eq{"exchange": options.Sessions})
	}

	var orderByColumn string
	switch options.OrderByColumn {
	case "":
		orderByColumn = "traded_at"
	case "traded_at", "gid":
		orderByColumn = options.OrderByColumn
	default:
		return nil, fmt.Errorf("invalid order by column: %s", options.OrderByColumn)
	}

	var ordering string

	switch strings.ToUpper(options.Ordering) {
	case "":
		ordering = "ASC"
	case "ASC", "DESC":
		ordering = strings.ToUpper(options.Ordering)
	default:
		return nil, fmt.Errorf("invalid ordering: %s", options.Ordering)
	}

	sel = sel.OrderBy(orderByColumn + " " + ordering)

	if options.Limit > 0 {
		sel = sel.Limit(options.Limit)
	}

	sql, args, err := sel.ToSql()
	if err != nil {
		return nil, err
	}

	log.Debug(sql)
	log.Debug(args)

	rows, err := s.DB.Queryx(sql, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
}

func (s *TradeService) Load(ctx context.Context, id int64) (*types.Trade, error) {
	var trade types.Trade
	query := "SELECT " + strings.Join(genTradeSelectColumns(s.DB.DriverName()), ", ") + " FROM trades WHERE id = :id"
	rows, err := s.DB.NamedQueryContext(ctx, query, map[string]interface{}{
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

func queryTradesSQL(options QueryTradesOptions) string {
	ordering := "ASC"
	switch v := strings.ToUpper(options.Ordering); v {
	case "DESC", "ASC":
		ordering = v
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

	if len(options.Symbol) > 0 {
		where = append(where, `symbol = :symbol`)
	}

	if len(options.Exchange) > 0 {
		where = append(where, `exchange = :exchange`)
	}

	sql := `SELECT * FROM trades`
	if len(where) > 0 {
		sql += ` WHERE ` + strings.Join(where, " AND ")
	}

	sql += ` ORDER BY gid ` + ordering

	if options.Limit > 0 {
		sql += ` LIMIT ` + strconv.FormatUint(options.Limit, 10)
	}

	return sql
}

func genTradeSelectColumns(driver string) []string {
	if driver != "mysql" {
		return []string{"*"}
	}
	tt := reflect.TypeOf(types.Trade{})
	var columns []string
	for i := 0; i < tt.NumField(); i++ {
		field := tt.Field(i)
		if colName := field.Tag.Get("db"); colName != "" {
			if colName == "-" {
				continue
			}
			if colName == "order_uuid" {
				// Use dialect-aware UUID selector for cross-database compatibility
				dialect := GetDialect(driver)
				columns = append(columns, dialectUuidSelector(dialect, "trades", "order_uuid"))
			} else {
				columns = append(columns, colName)
			}
		}
	}
	return columns
}

func (s *TradeService) scanRows(rows *sqlx.Rows) (trades []types.Trade, err error) {
	for rows.Next() {
		var trade types.Trade
		if err := rows.StructScan(&trade); err != nil {
			return trades, err
		}

		trades = append(trades, trade)
	}

	return trades, rows.Err()
}

func (s *TradeService) Insert(trade types.Trade) error {
    d := s.ensureDialect()
    insertClause := "id, order_id, order_uuid, exchange, price, quantity, quote_quantity, symbol, side, is_buyer, is_maker, traded_at, fee, fee_currency, is_margin, is_futures, is_isolated, strategy, pnl"

    // Use dialect-aware UUID handling
    valuesClause := ":id, :order_id, " + d.IfExpr(":order_uuid != ''", d.UUIDBinaryConversion(":order_uuid"), "''") + ", :exchange, :price, :quantity, :quote_quantity, :symbol, :side, :is_buyer, :is_maker, :traded_at, :fee, :fee_currency, :is_margin, :is_futures, :is_isolated, :strategy, :pnl"

    updateClause := "id=:id, order_id=:order_id, order_uuid=:order_uuid, exchange=:exchange, price=:price, quantity=:quantity, quote_quantity=:quote_quantity, symbol=:symbol, side=:side, is_buyer=:is_buyer, is_maker=:is_maker, traded_at=:traded_at, fee=:fee, fee_currency=:fee_currency, is_margin=:is_margin, is_futures=:is_futures, is_isolated=:is_isolated, strategy=:strategy, pnl=:pnl"

    sql := d.TradeUpsertSQL(insertClause, valuesClause, updateClause)
    _, err := s.DB.NamedExec(sql, trade)
    return err
}

func (s *TradeService) DeleteAll() error {
	_, err := s.DB.Exec(`DELETE FROM trades`)
	return err
}

func SelectLastTrades(ex types.ExchangeName, symbol string, isMargin, isFutures, isIsolated bool, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("trades").
		Where(sq.And{
			sq.Eq{"symbol": symbol},
			sq.Eq{"exchange": ex},
			sq.Eq{"is_margin": isMargin},
			sq.Eq{"is_futures": isFutures},
			sq.Eq{"is_isolated": isIsolated},
		}).
		OrderBy("traded_at DESC").
		Limit(limit)
}
