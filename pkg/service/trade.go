package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

var ErrTradeNotFound = errors.New("trade not found")

type QueryTradesOptions struct {
	Exchange types.ExchangeName
	Symbol   string
	LastGID  int64

	// ASC or DESC
	Ordering string
	Limit    int
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

	// records descending ordered, buffer 50 trades and use the trades ID to scan if the new trades are duplicated
	records, err := s.QueryLast(exchange.Name(), symbol, isMargin, isFutures, isIsolated, 50)
	if err != nil {
		return err
	}

	var tradeKeys = map[types.TradeKey]struct{}{}
	var lastTradeID uint64 = 1
	var now = time.Now()
	if len(records) > 0 {
		for _, record := range records {
			tradeKeys[record.Key()] = struct{}{}
		}

		lastTradeID = records[0].ID
		startTime = time.Time(records[0].Time)
	}

	b := &batch.TradeBatchQuery{Exchange: exchange}
	tradeC, errC := b.Query(ctx, symbol, &types.TradeQueryOptions{
		LastTradeID: lastTradeID,
		StartTime:   &startTime,
		EndTime:     &now,
	})

	for trade := range tradeC {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-errC:
			if err != nil {
				return err
			}

		default:
		}

		key := trade.Key()
		if _, exists := tradeKeys[key]; exists {
			continue
		}

		tradeKeys[key] = struct{}{}

		log.Infof("inserting trade: %s %d %s %-4s price: %-13v volume: %-11v %5s %s",
			trade.Exchange,
			trade.ID,
			trade.Symbol,
			trade.Side,
			trade.Price,
			trade.Quantity,
			trade.Liquidity(),
			trade.Time.String())

		if err := s.Insert(trade); err != nil {
			return err
		}
	}

	return <-errC
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

		record.Time = time.Date(record.Year, time.Month(record.Month), record.Day, 0, 0, 0, 0, time.UTC)
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

// QueryLast queries the last trade from the database
func (s *TradeService) QueryLast(ex types.ExchangeName, symbol string, isMargin, isFutures, isIsolated bool, limit int) ([]types.Trade, error) {
	log.Debugf("querying last trade exchange = %s AND symbol = %s AND is_margin = %v AND is_futures = %v AND is_isolated = %v", ex, symbol, isMargin, isFutures, isIsolated)

	sql := "SELECT * FROM trades WHERE exchange = :exchange AND symbol = :symbol AND is_margin = :is_margin AND is_futures = :is_futures AND is_isolated = :is_isolated ORDER BY gid DESC LIMIT :limit"
	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"symbol":      symbol,
		"exchange":    ex,
		"is_margin":   isMargin,
		"is_futures":  isFutures,
		"is_isolated": isIsolated,
		"limit":       limit,
	})
	if err != nil {
		return nil, errors.Wrap(err, "query last trade error")
	}

	defer rows.Close()

	return s.scanRows(rows)
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
	sql := queryTradesSQL(options)

	log.Debug(sql)

	args := map[string]interface{}{
		"exchange": options.Exchange,
		"symbol":   options.Symbol,
	}
	rows, err := s.DB.NamedQuery(sql, args)
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

func (s *TradeService) Mark(ctx context.Context, id int64, strategyID string) error {
	result, err := s.DB.NamedExecContext(ctx, "UPDATE `trades` SET `strategy` = :strategy WHERE `id` = :id", map[string]interface{}{
		"id":       id,
		"strategy": strategyID,
	})
	if err != nil {
		return err
	}

	cnt, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if cnt == 0 {
		return fmt.Errorf("trade id:%d not found", id)
	}

	return nil
}

func (s *TradeService) UpdatePnL(ctx context.Context, id int64, pnl float64) error {
	result, err := s.DB.NamedExecContext(ctx, "UPDATE `trades` SET `pnl` = :pnl WHERE `id` = :id", map[string]interface{}{
		"id":  id,
		"pnl": pnl,
	})
	if err != nil {
		return err
	}

	cnt, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if cnt == 0 {
		return fmt.Errorf("trade id:%d not found", id)
	}

	return nil

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
		sql += ` LIMIT ` + strconv.Itoa(options.Limit)
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
	_, err := s.DB.NamedExec(`
			INSERT INTO trades (
				id,
				exchange, 
				order_id,
				symbol,
				price,
				quantity,
				quote_quantity,
				side,
				is_buyer,
				is_maker,
				fee,
				fee_currency,
				traded_at,
				is_margin,
				is_futures,
				is_isolated)
			VALUES (
				:id,
				:exchange,
				:order_id,
				:symbol,
				:price,
				:quantity,
				:quote_quantity,
				:side,
				:is_buyer,
				:is_maker,
				:fee,
				:fee_currency,
				:traded_at,
				:is_margin,
				:is_futures,
				:is_isolated
			)`,
		trade)
	return err
}

func (s *TradeService) DeleteAll() error {
	_, err := s.DB.Exec(`DELETE FROM trades`)
	return err
}
