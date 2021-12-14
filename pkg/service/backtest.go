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

	batch2 "github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

type BacktestService struct {
	DB *sqlx.DB
}

func (s *BacktestService) SyncKLineByInterval(ctx context.Context, exchange types.Exchange, symbol string, interval types.Interval, startTime, endTime time.Time) error {
	log.Infof("synchronizing lastKLine for interval %s from exchange %s", interval, exchange.Name())

	lastKLine, err := s.QueryKLine(exchange.Name(), symbol, interval, "DESC", 1)
	if err != nil {
		return err
	}

	if lastKLine != nil {
		log.Infof("found the last %s kline data checkpoint %s", symbol, lastKLine.EndTime)
		startTime = lastKLine.StartTime.Add(time.Minute)
	}

	batch := &batch2.KLineBatchQuery{Exchange: exchange}

	// should use channel here
	klineC, errC := batch.Query(ctx, symbol, interval, startTime, endTime)

	// var previousKLine types.KLine
	count := 0
	for klines := range klineC {
		if err := s.BatchInsert(klines); err != nil {
			return err
		}
		count += len(klines)
	}
	log.Infof("found %s kline %s data count: %d", symbol, interval.String(), count)

	if err := <-errC; err != nil {
		return err
	}

	return nil
}

func (s *BacktestService) Sync(ctx context.Context, exchange types.Exchange, symbol string, startTime time.Time) error {
	endTime := time.Now()

	exCustom, ok := exchange.(types.CustomIntervalProvider)

	var supportIntervals map[types.Interval]int
	if ok {
		supportIntervals = exCustom.SupportedInterval()
	} else {
		supportIntervals = types.SupportedIntervals
	}

	for interval := range supportIntervals {
		if err := s.SyncKLineByInterval(ctx, exchange, symbol, interval, startTime, endTime); err != nil {
			return err
		}
	}

	return nil
}

func (s *BacktestService) QueryFirstKLine(ex types.ExchangeName, symbol string, interval types.Interval) (*types.KLine, error) {
	return s.QueryKLine(ex, symbol, interval, "ASC", 1)
}

// QueryLastKLine queries the last kline from the database
func (s *BacktestService) QueryLastKLine(ex types.ExchangeName, symbol string, interval types.Interval) (*types.KLine, error) {
	return s.QueryKLine(ex, symbol, interval, "DESC", 1)
}

// QueryKLine queries the klines from the database
func (s *BacktestService) QueryKLine(ex types.ExchangeName, symbol string, interval types.Interval, orderBy string, limit int) (*types.KLine, error) {
	log.Infof("querying last kline exchange = %s AND symbol = %s AND interval = %s", ex, symbol, interval)

	tableName := s._targetKlineTable(ex)
	// make the SQL syntax IDE friendly, so that it can analyze it.
	sql := fmt.Sprintf("SELECT * FROM `%s` WHERE  `symbol` = :symbol AND `interval` = :interval  and exchange = :exchange  ORDER BY end_time "+orderBy+" LIMIT "+strconv.Itoa(limit), tableName)

	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"exchange": ex.String(),
		"interval": interval,
		"symbol":   symbol,
	})

	if err != nil {
		return nil, errors.Wrap(err, "query kline error")
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	defer rows.Close()

	if rows.Next() {
		var kline types.KLine
		err = rows.StructScan(&kline)
		return &kline, err
	}

	return nil, rows.Err()
}

func (s *BacktestService) QueryKLinesForward(exchange types.ExchangeName, symbol string, interval types.Interval, startTime time.Time, limit int) ([]types.KLine, error) {
	tableName := s._targetKlineTable(exchange)
	sql := "SELECT * FROM `binance_klines` WHERE `end_time` >= :start_time AND `symbol` = :symbol AND `interval` = :interval and exchange = :exchange ORDER BY end_time ASC LIMIT :limit"
	sql = strings.ReplaceAll(sql, "binance_klines", tableName)

	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"start_time": startTime,
		"limit":      limit,
		"symbol":     symbol,
		"interval":   interval,
		"exchange":   exchange.String(),
	})
	if err != nil {
		return nil, err
	}

	return s.scanRows(rows)
}

func (s *BacktestService) QueryKLinesBackward(exchange types.ExchangeName, symbol string, interval types.Interval, endTime time.Time, limit int) ([]types.KLine, error) {
	tableName := s._targetKlineTable(exchange)

	sql := "SELECT * FROM `binance_klines` WHERE `end_time` <= :end_time  and exchange = :exchange  AND `symbol` = :symbol AND `interval` = :interval ORDER BY end_time DESC LIMIT :limit"
	sql = strings.ReplaceAll(sql, "binance_klines", tableName)
	sql = "SELECT t.* FROM (" + sql + ") AS t ORDER BY t.end_time ASC"

	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"limit":    limit,
		"end_time": endTime,
		"symbol":   symbol,
		"interval": interval,
		"exchange": exchange.String(),
	})
	if err != nil {
		return nil, err
	}

	return s.scanRows(rows)
}

func (s *BacktestService) QueryKLinesCh(since, until time.Time, exchange types.Exchange, symbols []string, intervals []types.Interval) (chan types.KLine, chan error) {

	if len(symbols) == 0 {

		errC := make(chan error, 1)
		// avoid blocking
		go func() {
			errC <- errors.Errorf("symbols is empty when querying kline, plesae check your strategy setting. ")
			close(errC)
		}()
		return nil, errC
	}

	tableName := s._targetKlineTable(exchange.Name())
	sql := "SELECT * FROM `binance_klines` WHERE `end_time` BETWEEN :since AND :until AND `symbol` IN (:symbols) AND `interval` IN (:intervals)  and exchange = :exchange  ORDER BY end_time ASC"
	sql = strings.ReplaceAll(sql, "binance_klines", tableName)

	sql, args, err := sqlx.Named(sql, map[string]interface{}{
		"since":     since,
		"until":     until,
		"symbols":   symbols,
		"intervals": types.IntervalSlice(intervals),
		"exchange":  exchange.Name().String(),
	})

	sql, args, err = sqlx.In(sql, args...)
	sql = s.DB.Rebind(sql)

	rows, err := s.DB.Queryx(sql, args...)
	if err != nil {
		log.WithError(err).Error("backtest query error")

		errC := make(chan error, 1)
		// avoid blocking
		go func() {
			errC <- err
			close(errC)
		}()
		return nil, errC
	}

	return s.scanRowsCh(rows)
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
	for rows.Next() {
		var kline types.KLine
		if err := rows.StructScan(&kline); err != nil {
			return nil, err
		}

		klines = append(klines, kline)
	}

	return klines, rows.Err()
}

func (s *BacktestService) _targetKlineTable(exchangeName types.ExchangeName) string {
	switch exchangeName {
	case types.ExchangeBinance:
		return "binance_klines"
	case types.ExchangeFTX:
		return "ftx_klines"
	case types.ExchangeMax:
		return "max_klines"
	case types.ExchangeOKEx:
		return "okex_klines"
	default:
		return "klines"
	}
}

func (s *BacktestService) Insert(kline types.KLine) error {
	if len(kline.Exchange) == 0 {
		return errors.New("kline.Exchange field should not be empty")
	}

	tableName := s._targetKlineTable(kline.Exchange)

	sql := fmt.Sprintf("INSERT INTO `%s` (`exchange`, `start_time`, `end_time`, `symbol`, `interval`, `open`, `high`, `low`, `close`, `closed`, `volume`, `quote_volume`, `taker_buy_base_volume`, `taker_buy_quote_volume`)"+
		"VALUES (:exchange, :start_time, :end_time, :symbol, :interval, :open, :high, :low, :close, :closed, :volume, :quote_volume, :taker_buy_base_volume, :taker_buy_quote_volume)", tableName)

	_, err := s.DB.NamedExec(sql, kline)
	return err
}

// BatchInsert Note: all kline should be same exchange, or it will cause issue.
func (s *BacktestService) BatchInsert(kline []types.KLine) error {
	if len(kline) == 0 {
		return nil
	}
	if len(kline[0].Exchange) == 0 {
		return errors.New("kline.Exchange field should not be empty")
	}

	tableName := s._targetKlineTable(kline[0].Exchange)

	sql := fmt.Sprintf("INSERT INTO `%s` (`exchange`, `start_time`, `end_time`, `symbol`, `interval`, `open`, `high`, `low`, `close`, `closed`, `volume`, `quote_volume`, `taker_buy_base_volume`, `taker_buy_quote_volume`)"+
		" values (:exchange, :start_time, :end_time, :symbol, :interval, :open, :high, :low, :close, :closed, :volume, :quote_volume, :taker_buy_base_volume, :taker_buy_quote_volume); ", tableName)

	_, err := s.DB.NamedExec(sql, kline)
	return err
}

func (s *BacktestService) DeleteDuplicatedKLine(k types.KLine) error {

	if len(k.Exchange) == 0 {
		return errors.New("kline.Exchange field should not be empty")
	}

	tableName := s._targetKlineTable(k.Exchange)
	sql := fmt.Sprintf("delete from `%s` where gid = :gid  ", tableName)
	_, err := s.DB.NamedExec(sql, k)
	return err
}
