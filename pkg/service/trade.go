package service

import (
	"context"
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
)

var ErrTradeNotFound = errors.New("trade not found")

type QueryTradesOptions struct {
	Exchange types.ExchangeName
	Sessions []string
	Symbol   string
	LastGID  int64
	Since    *time.Time

	// ASC or DESC
	Ordering string
	Limit    uint64
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
	DB *sqlx.DB
}

func NewTradeService(db *sqlx.DB) *TradeService {
	return &TradeService{db}
}

func (s *TradeService) Sync(ctx context.Context, exchange types.Exchange, symbol string, startTime time.Time) error {
	isMargin, isFutures, isIsolated, isolatedSymbol := exchange2.GetSessionAttributes(exchange)
	// override symbol if isolatedSymbol is not empty
	if isIsolated && len(isolatedSymbol) > 0 {
		symbol = isolatedSymbol
	}

	api, ok := exchange.(types.ExchangeTradeHistoryService)
	if !ok {
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
			},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.TradeBatchQuery{
					ExchangeTradeHistoryService: api,
				}
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
				return strconv.FormatUint(trade.ID, 10) + trade.Side.String()
			},
			LogInsert: true,
		},
	}

	for _, sel := range tasks {
		if err := sel.execute(ctx, s.DB, startTime); err != nil {
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

	sql := ""
	driverName := s.DB.DriverName()
	if driverName == "mysql" {
		sql = generateMysqlTradingVolumeQuerySQL(options)
	} else {
		sql = generateSqliteTradingVolumeSQL(options)
	}

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

func generateSqliteTradingVolumeSQL(options TradingVolumeQueryOptions) string {
	timeRangeColumn := "traded_at"
	sel, groupBys, orderBys := generateSqlite3TimeRangeClauses(timeRangeColumn, options.GroupByPeriod)

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

func generateSqlite3TimeRangeClauses(timeRangeColumn, period string) (selectors []string, groupBys []string, orderBys []string) {
	switch period {
	case "month":
		selectors = append(selectors, "strftime('%Y',"+timeRangeColumn+") AS year", "strftime('%m',"+timeRangeColumn+") AS month")
		groupBys = append([]string{"month", "year"}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC")

	case "year":
		selectors = append(selectors, "strftime('%Y',"+timeRangeColumn+") AS year")
		groupBys = append([]string{"year"}, groupBys...)
		orderBys = append(orderBys, "year ASC")

	case "day":
		fallthrough

	default:
		selectors = append(selectors, "strftime('%Y',"+timeRangeColumn+") AS year", "strftime('%m',"+timeRangeColumn+") AS month", "strftime('%d',"+timeRangeColumn+") AS day")
		groupBys = append([]string{"day", "month", "year"}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC", "day ASC")
	}

	return
}

func generateMysqlTimeRangeClauses(timeRangeColumn, period string) (selectors []string, groupBys []string, orderBys []string) {
	switch period {
	case "month":
		selectors = append(selectors, "YEAR("+timeRangeColumn+") AS year", "MONTH("+timeRangeColumn+") AS month")
		groupBys = append([]string{"MONTH(" + timeRangeColumn + ")", "YEAR(" + timeRangeColumn + ")"}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC")

	case "year":
		selectors = append(selectors, "YEAR("+timeRangeColumn+") AS year")
		groupBys = append([]string{"YEAR(" + timeRangeColumn + ")"}, groupBys...)
		orderBys = append(orderBys, "year ASC")

	case "day":
		fallthrough

	default:
		selectors = append(selectors, "YEAR("+timeRangeColumn+") AS year", "MONTH("+timeRangeColumn+") AS month", "DAY("+timeRangeColumn+") AS day")
		groupBys = append([]string{"DAY(" + timeRangeColumn + ")", "MONTH(" + timeRangeColumn + ")", "YEAR(" + timeRangeColumn + ")"}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC", "day ASC")
	}

	return
}

func generateMysqlTradingVolumeQuerySQL(options TradingVolumeQueryOptions) string {
	timeRangeColumn := "traded_at"
	sel, groupBys, orderBys := generateMysqlTimeRangeClauses(timeRangeColumn, options.GroupByPeriod)

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
	sql := "SELECT * FROM trades WHERE exchange = :exchange AND (symbol = :symbol OR fee_currency = :fee_currency) ORDER BY traded_at ASC"
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
	sel := sq.Select("*").
		From("trades")

	if options.Since != nil {
		sel = sel.Where(sq.GtOrEq{"traded_at": options.Since})
	}

	sel = sel.Where(sq.Eq{"symbol": options.Symbol})

	if options.Exchange != "" {
		sel = sel.Where(sq.Eq{"exchange": options.Exchange})
	}

	if len(options.Sessions) > 0 {
		// FIXME: right now we only have the exchange field in the db, we might need to add the session field too.
		sel = sel.Where(sq.Eq{"exchange": options.Sessions})
	}

	if options.Ordering != "" {
		sel = sel.OrderBy("traded_at " + options.Ordering)
	} else {
		sel = sel.OrderBy("traded_at ASC")
	}

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
	sql := dbCache.InsertSqlOf(trade)
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

