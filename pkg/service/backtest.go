package service

import (
	"context"
	"database/sql"
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

type BacktestService struct {
    DB      *sqlx.DB
    dialect DatabaseDialect
}

func NewBacktestService(db *sqlx.DB) *BacktestService {
    return &BacktestService{
        DB:      db,
        dialect: GetDialect(db.DriverName()),
    }
}

func (s *BacktestService) ensureDialect() DatabaseDialect {
    if s.dialect == nil {
        s.dialect = GetDialect(s.DB.DriverName())
    }
    return s.dialect
}

func (s *BacktestService) SyncKLineByInterval(
    ctx context.Context, exchange types.Exchange, symbol string, interval types.Interval, startTime, endTime time.Time,
) error {
    s.ensureDialect()
	_, isFutures, isIsolated, isolatedSymbol := exchange2.GetSessionAttributes(exchange)

	// override symbol if isolatedSymbol is not empty
	if isIsolated && len(isolatedSymbol) > 0 {
		symbol = isolatedSymbol
	}

	if isFutures {
		log.Infof("synchronizing %s futures klines with interval %s: %s <=> %s", exchange.Name(), interval, startTime, endTime)
	} else {
		log.Infof("synchronizing %s klines with interval %s: %s <=> %s", exchange.Name(), interval, startTime, endTime)
	}

	if s.DB.DriverName() == "sqlite3" {
		_, _ = s.DB.Exec("PRAGMA journal_mode = WAL")
		_, _ = s.DB.Exec("PRAGMA synchronous = NORMAL")
	}

	now := time.Now()
	tasks := []SyncTask{
		{
			Type:   types.KLine{},
			Select: s.SelectLastKLines(exchange, symbol, interval, startTime, endTime, 100),
			Time: func(obj interface{}) time.Time {
				return obj.(types.KLine).StartTime.Time()
			},
			Filter: func(obj interface{}) bool {
				k := obj.(types.KLine)
				if k.EndTime.Before(k.StartTime.Time().Add(k.Interval.Duration() - time.Second)) {
					return false
				}

				// Filter klines that has the endTime closed in the future
				if k.EndTime.After(now) {
					return false
				}

				return true
			},
			ID: func(obj interface{}) string {
				kline := obj.(types.KLine)
				return strconv.FormatInt(kline.StartTime.UnixMilli(), 10)
				// return kline.Symbol + kline.Interval.String() + strconv.FormatInt(kline.StartTime.UnixMilli(), 10)
			},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				q := &batch.KLineBatchQuery{Exchange: exchange}
				return q.Query(ctx, symbol, interval, startTime, endTime)
			},
			BatchInsertBuffer: 1000,
			BatchInsert: func(obj interface{}) error {
				kLines := obj.([]types.KLine)
				return s.BatchInsert(kLines, exchange)
			},
			Insert: func(obj interface{}) error {
				kline := obj.(types.KLine)
				return s.Insert(kline, exchange)
			},
			LogInsert: log.GetLevel() == log.DebugLevel,
		},
	}

	for _, sel := range tasks {
		if err := sel.execute(ctx, s.DB, startTime, endTime); err != nil {
			return err
		}
	}

	return nil
}

func (s *BacktestService) Verify(sourceExchange types.Exchange, symbols []string, startTime time.Time, endTime time.Time) error {
	var corruptCnt = 0
	for _, symbol := range symbols {
		for interval := range types.SupportedIntervals {
			log.Infof("verifying %s %s backtesting data: %s to %s...", symbol, interval, startTime, endTime)

			timeRanges, err := s.FindMissingTimeRanges(context.Background(), sourceExchange, symbol, interval,
				startTime, endTime)
			if err != nil {
				return err
			}

			if len(timeRanges) == 0 {
				continue
			}

			log.Warnf("%s %s found missing time ranges:", symbol, interval)
			corruptCnt += len(timeRanges)
			for _, timeRange := range timeRanges {
				log.Warnf("- %s", timeRange.String())
			}
		}
	}

	log.Infof("backtest verification completed")
	if corruptCnt > 0 {
		log.Errorf("found %d corruptions", corruptCnt)
	} else {
		log.Infof("found %d corruptions", corruptCnt)
	}

	return nil
}

func (s *BacktestService) SyncFresh(
	ctx context.Context, exchange types.Exchange, symbol string, interval types.Interval, startTime, endTime time.Time,
) error {
	log.Infof("starting fresh sync %s %s %s: %s <=> %s", exchange.Name(), symbol, interval, startTime, endTime)
	startTime = startTime.Truncate(time.Minute).Add(-2 * time.Second)
	endTime = endTime.Truncate(time.Minute).Add(2 * time.Second)
	return s.SyncKLineByInterval(ctx, exchange, symbol, interval, startTime, endTime)
}

// QueryKLine queries the klines from the database
func (s *BacktestService) QueryKLine(
    ex types.Exchange, symbol string, interval types.Interval, orderBy string, limit int,
) (*types.KLine, error) {
    s.ensureDialect()
	log.Infof("querying last kline exchange = %s AND symbol = %s AND interval = %s", ex, symbol, interval)

	tableName := targetKlineTable(ex)

	// Build query using Squirrel for proper cross-database compatibility
	query := sq.Select("*").
		From(tableName).
		Where(sq.Eq{"symbol": symbol}).
		Where(sq.Eq{s.dialect.EscapeColumnName("interval"): interval.String()}).
		OrderBy("end_time " + orderBy).
		Limit(uint64(limit))

	// Configure database-specific placeholder format
	query = s.dialect.ConfigurePlaceholder(query)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build query error")
	}

	rows, err := s.DB.Queryx(sql, args...)
	if err != nil {
		return nil, errors.Wrap(err, "query kline error")
	}
	defer rows.Close()

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if rows.Next() {
		var kline types.KLine
		err = rows.StructScan(&kline)
		return &kline, err
	}

	return nil, rows.Err()
}

// QueryKLinesForward is used for querying klines to back-testing
func (s *BacktestService) QueryKLinesForward(
    exchange types.Exchange, symbol string, interval types.Interval, startTime time.Time, limit int,
) ([]types.KLine, error) {
    s.ensureDialect()
	tableName := targetKlineTable(exchange)

	// Build query using Squirrel for proper cross-database compatibility
	query := sq.Select("*").
		From(tableName).
		Where(sq.GtOrEq{"end_time": startTime}).
		Where(sq.Eq{"symbol": symbol}).
		Where(sq.Eq{s.dialect.EscapeColumnName("interval"): interval.String()}).
		Where(sq.Eq{"exchange": exchange.Name().String()}).
		OrderBy("end_time ASC").
		Limit(uint64(limit))

	// Configure database-specific placeholder format
	query = s.dialect.ConfigurePlaceholder(query)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := s.DB.Queryx(sql, args...)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	return s.scanRows(rows)
}

func (s *BacktestService) QueryKLinesBackward(
    exchange types.Exchange, symbol string, interval types.Interval, endTime time.Time, limit int,
) ([]types.KLine, error) {
    s.ensureDialect()
	tableName := targetKlineTable(exchange)

	// Build subquery using Squirrel for proper cross-database compatibility
	subQuery := sq.Select("*").
		From(tableName).
		Where(sq.LtOrEq{"end_time": endTime}).
		Where(sq.Eq{"exchange": exchange.Name().String()}).
		Where(sq.Eq{"symbol": symbol}).
		Where(sq.Eq{s.dialect.EscapeColumnName("interval"): interval.String()}).
		OrderBy("end_time DESC").
		Limit(uint64(limit))

	// Configure database-specific placeholder format for subquery
	subQuery = s.dialect.ConfigurePlaceholder(subQuery)

	subSql, subArgs, err := subQuery.ToSql()
	if err != nil {
		return nil, err
	}

	// Wrap in outer query to reverse order
	// Note: Using raw SQL here as Squirrel doesn't handle subqueries in FROM clause well
	sql := "SELECT t.* FROM (" + subSql + ") AS t ORDER BY t.end_time ASC"

	rows, err := s.DB.Queryx(sql, subArgs...)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	return s.scanRows(rows)
}

func (s *BacktestService) QueryKLinesCh(
    since, until time.Time, exchange types.Exchange, symbols []string, intervals []types.Interval,
) (chan types.KLine, chan error) {
    s.ensureDialect()
	if len(symbols) == 0 {
		return returnError(errors.Errorf("symbols is empty when querying kline, plesae check your strategy setting. "))
	}

	tableName := targetKlineTable(exchange)

	// Convert intervals to strings for query
	intervalStrs := make([]string, len(intervals))
	for i, interval := range intervals {
		intervalStrs[i] = interval.String()
	}

	// Build query using Squirrel for proper cross-database compatibility
	query := sq.Select("*").
		From(tableName).
		Where(sq.And{
			sq.GtOrEq{"end_time": since},
			sq.LtOrEq{"end_time": until},
		}).
		OrderBy("end_time ASC", "start_time DESC")

	// Handle symbols - use single value if only one symbol, otherwise use IN clause
	if len(symbols) == 1 {
		query = query.Where(sq.Eq{"symbol": symbols[0]})
	} else {
		query = query.Where(sq.Eq{"symbol": symbols})
	}

	// Handle intervals - always use IN clause since we have multiple intervals
	query = query.Where(sq.Eq{s.dialect.EscapeColumnName("interval"): intervalStrs})

	// Configure database-specific placeholder format
	query = s.dialect.ConfigurePlaceholder(query)

	sql, args, err := query.ToSql()
	if err != nil {
		return returnError(err)
	}

	rows, err := s.DB.Queryx(sql, args...)
	if err != nil {
		return returnError(err)
	}

	return s.scanRowsCh(rows)
}

func returnError(err error) (chan types.KLine, chan error) {
	ch := make(chan types.KLine)
	close(ch)
	log.WithError(err).Error("backtest query error")

	errC := make(chan error, 1)
	// avoid blocking
	go func() {
		errC <- err
		close(errC)
	}()
	return ch, errC
}

// scanRowsCh scan rows into channel
func (s *BacktestService) scanRowsCh(rows *sqlx.Rows) (chan types.KLine, chan error) {
	ch := make(chan types.KLine, 500)
	errC := make(chan error, 1)

	go func() {
		defer close(errC)
		defer close(ch)
		defer rows.Close()

		for rows.Next() {
			var kline types.KLine
			if err := rows.StructScan(&kline); err != nil {
				errC <- err
				return
			}

			ch <- kline
		}

		if err := rows.Err(); err != nil {
			errC <- err
			return
		}

	}()

	return ch, errC
}

func (s *BacktestService) scanRows(rows *sqlx.Rows) (klines []types.KLine, err error) {
	defer rows.Close()
	for rows.Next() {
		var kline types.KLine
		if err := rows.StructScan(&kline); err != nil {
			return nil, err
		}

		klines = append(klines, kline)
	}

	return klines, rows.Err()
}

func targetKlineTable(exchange types.Exchange) string {
	_, isFutures, _, _ := exchange2.GetSessionAttributes(exchange)

	tableName := strings.ToLower(exchange.Name().String())
	if isFutures {
		return tableName + "_futures_klines"
	} else {
		return tableName + "_klines"
	}
}

var errExchangeFieldIsUnset = errors.New("kline.Exchange field should not be empty")

func (s *BacktestService) Insert(kline types.KLine, ex types.Exchange) error {
    s.ensureDialect()
	if len(kline.Exchange) == 0 {
		return errExchangeFieldIsUnset
	}

	tableName := targetKlineTable(ex)

	// Generate the appropriate SQL using dialect for cross-database compatibility
	columns := []string{
		s.dialect.EscapeColumnName("exchange"), s.dialect.EscapeColumnName("start_time"), s.dialect.EscapeColumnName("end_time"), s.dialect.EscapeColumnName("symbol"),
		s.dialect.EscapeColumnName("interval"), s.dialect.EscapeColumnName("open"), s.dialect.EscapeColumnName("high"), s.dialect.EscapeColumnName("low"),
		s.dialect.EscapeColumnName("close"), s.dialect.EscapeColumnName("closed"), s.dialect.EscapeColumnName("volume"), s.dialect.EscapeColumnName("quote_volume"),
		s.dialect.EscapeColumnName("taker_buy_base_volume"), s.dialect.EscapeColumnName("taker_buy_quote_volume"),
	}
	insertClause := strings.Join(columns, ", ")
	valuesClause := ":exchange, :start_time, :end_time, :symbol, :interval, :open, :high, :low, :close, :closed, :volume, :quote_volume, :taker_buy_base_volume, :taker_buy_quote_volume"

	// Use dialect-specific INSERT with conflict handling (ignore duplicates)
	sql := s.dialect.KLineInsertSQL(s.dialect.EscapeTableName(tableName), insertClause, valuesClause)

	_, err := s.DB.NamedExec(sql, kline)
	return err
}

// BatchInsert Note: all kline should be same exchange, or it will cause issue.
func (s *BacktestService) BatchInsert(kline []types.KLine, ex types.Exchange) error {
    s.ensureDialect()
	if len(kline) == 0 {
		return nil
	}

	tableName := targetKlineTable(ex)

	// Generate the appropriate SQL using dialect for cross-database compatibility
	columns := []string{
		s.dialect.EscapeColumnName("exchange"), s.dialect.EscapeColumnName("start_time"), s.dialect.EscapeColumnName("end_time"), s.dialect.EscapeColumnName("symbol"),
		s.dialect.EscapeColumnName("interval"), s.dialect.EscapeColumnName("open"), s.dialect.EscapeColumnName("high"), s.dialect.EscapeColumnName("low"),
		s.dialect.EscapeColumnName("close"), s.dialect.EscapeColumnName("closed"), s.dialect.EscapeColumnName("volume"), s.dialect.EscapeColumnName("quote_volume"),
		s.dialect.EscapeColumnName("taker_buy_base_volume"), s.dialect.EscapeColumnName("taker_buy_quote_volume"),
	}
	insertClause := strings.Join(columns, ", ")
	valuesClause := ":exchange, :start_time, :end_time, :symbol, :interval, :open, :high, :low, :close, :closed, :volume, :quote_volume, :taker_buy_base_volume, :taker_buy_quote_volume"

	// Use dialect-specific INSERT with conflict handling (ignore duplicates)
	sql := s.dialect.KLineInsertSQL(s.dialect.EscapeTableName(tableName), insertClause, valuesClause)

	tx := s.DB.MustBegin()
	if _, err := tx.NamedExec(sql, kline); err != nil {
		if e := tx.Rollback(); e != nil {
			log.WithError(e).Fatalf("cannot rollback insertion %v", err)
		}
		return err
	}
	return tx.Commit()
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

func (t *TimeRange) String() string {
	return t.Start.String() + " ~ " + t.End.String()
}

func (s *BacktestService) Sync(
	ctx context.Context, ex types.Exchange, symbol string, interval types.Interval, since, until time.Time,
) error {
	t1, t2, err := s.QueryExistingDataRange(ctx, ex, symbol, interval, since, until)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows || t1 == nil || t2 == nil {
		// fallback to fresh sync
		return s.SyncFresh(ctx, ex, symbol, interval, since, until)
	}

	return s.SyncPartial(ctx, ex, symbol, interval, since, until)
}

// SyncPartial
// find the existing data time range (t1, t2)
// scan if there is a missing part
// create a time range slice []TimeRange
// iterate the []TimeRange slice to sync data.
func (s *BacktestService) SyncPartial(
	ctx context.Context, ex types.Exchange, symbol string, interval types.Interval, since, until time.Time,
) error {
	log.Infof("starting partial sync %s %s %s: %s <=> %s", ex.Name(), symbol, interval, since, until)

	t1, t2, err := s.QueryExistingDataRange(ctx, ex, symbol, interval, since, until)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows || t1 == nil || t2 == nil {
		// fallback to fresh sync
		return s.SyncFresh(ctx, ex, symbol, interval, since, until)
	}

	timeRanges, err := s.FindMissingTimeRanges(ctx, ex, symbol, interval, t1.Time(), t2.Time())
	if err != nil {
		return err
	}

	if len(timeRanges) > 0 {
		log.Infof("found missing data time ranges: %v", timeRanges)
	}

	// there are few cases:
	// t1 == since && t2 == until
	// [since] ------- [t1] data [t2] ------ [until]
	if since.Before(t1.Time()) && t1.Time().Sub(since) > interval.Duration() {
		// shift slice
		timeRanges = append([]TimeRange{
			{Start: since.Add(-2 * time.Second), End: t1.Time()}, // we should include since
		}, timeRanges...)
	}

	if t2.Time().Before(until) && until.Sub(t2.Time()) > interval.Duration() {
		timeRanges = append(timeRanges, TimeRange{
			Start: t2.Time(),
			End:   until.Add(-interval.Duration()), // include until
		})
	}

	for _, timeRange := range timeRanges {
		err = s.SyncKLineByInterval(ctx, ex, symbol, interval, timeRange.Start.Add(time.Second), timeRange.End.Add(-time.Second))
		if err != nil {
			return err
		}
	}

	return nil
}

// FindMissingTimeRanges returns the missing time ranges, the start/end time represents the existing data time points.
// So when sending kline query to the exchange API, we need to add one second to the start time and minus one second to the end time.
func (s *BacktestService) FindMissingTimeRanges(
    ctx context.Context, ex types.Exchange, symbol string, interval types.Interval, since, until time.Time,
) ([]TimeRange, error) {
    s.ensureDialect()
	query := s.SelectKLineTimePoints(ex, symbol, interval, since, until)
	// Configure the correct placeholder format for the database driver
	query = s.dialect.ConfigurePlaceholder(query)
	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := s.DB.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timeRanges []TimeRange
	var lastTime = since
	var intervalDuration = interval.Duration()
	for rows.Next() {
		var tt types.Time
		if err := rows.Scan(&tt); err != nil {
			return nil, err
		}

		var t = time.Time(tt)
		if t.Sub(lastTime) > intervalDuration {
			timeRanges = append(timeRanges, TimeRange{
				Start: lastTime,
				End:   t,
			})
		}

		lastTime = t
	}

	if lastTime.Before(until) && until.Sub(lastTime) > intervalDuration {
		timeRanges = append(timeRanges, TimeRange{
			Start: lastTime,
			End:   until,
		})
	}

	return timeRanges, nil
}

func (s *BacktestService) QueryExistingDataRange(
    ctx context.Context, ex types.Exchange, symbol string, interval types.Interval, tArgs ...time.Time,
) (start, end *types.Time, err error) {
    s.ensureDialect()
	sel := s.SelectKLineTimeRange(ex, symbol, interval, tArgs...)
	// Configure the correct placeholder format for the database driver
	sel = s.dialect.ConfigurePlaceholder(sel)
	sql, args, err := sel.ToSql()
	if err != nil {
		return nil, nil, err
	}

	var t1, t2 types.Time

	row := s.DB.QueryRowContext(ctx, sql, args...)

	if err := row.Scan(&t1, &t2); err != nil {
		return nil, nil, err
	}

	if err := row.Err(); err != nil {
		return nil, nil, err
	}

	if t1.Time().IsZero() || t2.Time().IsZero() {
		return nil, nil, nil
	}

	return &t1, &t2, nil
}

func (s *BacktestService) SelectKLineTimePoints(
	ex types.Exchange, symbol string, interval types.Interval, args ...time.Time,
) sq.SelectBuilder {
	conditions := sq.And{
		sq.Eq{"symbol": symbol},
		sq.Eq{s.dialect.EscapeColumnName("interval"): interval.String()},
	}

	if len(args) == 2 {
		since := args[0]
		until := args[1]
		conditions = append(conditions, sq.Expr(s.dialect.EscapeColumnName("start_time")+" BETWEEN ? AND ?", since, until))
	}

	tableName := targetKlineTable(ex)

	sel := sq.Select("start_time").
		From(tableName).
		Where(conditions).
		OrderBy("start_time ASC")

	// Configure the correct placeholder format for the database driver
	return s.dialect.ConfigurePlaceholder(sel)
}

// SelectKLineTimeRange returns the existing klines time range (since < kline.start_time < until)
func (s *BacktestService) SelectKLineTimeRange(
	ex types.Exchange, symbol string, interval types.Interval, args ...time.Time,
) sq.SelectBuilder {
	conditions := sq.And{
		sq.Eq{"symbol": symbol},
		sq.Eq{s.dialect.EscapeColumnName("interval"): interval.String()},
	}

	if len(args) == 2 {
		// NOTE
		// sqlite does not support timezone format, so we are converting to local timezone
		// mysql works in this case, so this is a workaround
		since := args[0]
		until := args[1]
		conditions = append(conditions, sq.Expr(s.dialect.EscapeColumnName("start_time")+" BETWEEN ? AND ?", since, until))
	}

	tableName := targetKlineTable(ex)

	sel := sq.Select("MIN(start_time) AS t1, MAX(start_time) AS t2").
		From(tableName).
		Where(conditions)

	// Configure the correct placeholder format for the database driver
	return s.dialect.ConfigurePlaceholder(sel)
}

// TODO: add is_futures column since the klines data is different
func (s *BacktestService) SelectLastKLines(
	ex types.Exchange, symbol string, interval types.Interval, startTime, endTime time.Time, limit uint64,
) sq.SelectBuilder {
	tableName := targetKlineTable(ex)
	sel := sq.Select("*").
		From(tableName).
		Where(sq.And{
			sq.Eq{"symbol": symbol},
			sq.Eq{s.dialect.EscapeColumnName("interval"): interval.String()},
			sq.Expr(s.dialect.EscapeColumnName("start_time")+" BETWEEN ? AND ?", startTime, endTime),
		}).
		OrderBy("start_time DESC").
		Limit(limit)

	// Configure the correct placeholder format for the database driver
	return s.dialect.ConfigurePlaceholder(sel)
}
