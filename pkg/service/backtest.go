package service

import (
	"context"
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

	lastKLine, err := s.QueryLast(exchange.Name(), symbol, interval)
	if err != nil {
		return err
	}

	if lastKLine != nil {
		log.Infof("found last checkpoint %s", lastKLine.EndTime)
		startTime = lastKLine.StartTime.Add(time.Minute)
	}

	batch := &batch2.KLineBatchQuery{Exchange: exchange}

	// should use channel here
	klineC, errC := batch.Query(ctx, symbol, interval, startTime, endTime)
	// var previousKLine types.KLine
	for k := range klineC {
		if err := s.Insert(k); err != nil {
			return err
		}
	}

	if err := <-errC; err != nil {
		return err
	}

	return nil
}

func (s *BacktestService) Sync(ctx context.Context, exchange types.Exchange, symbol string, startTime time.Time) error {
	endTime := time.Now()
	for interval := range types.SupportedIntervals {
		if err := s.SyncKLineByInterval(ctx, exchange, symbol, interval, startTime, endTime); err != nil {
			return err
		}
	}

	return nil
}

// QueryLast queries the last order from the database
func (s *BacktestService) QueryLast(ex types.ExchangeName, symbol string, interval types.Interval) (*types.KLine, error) {
	log.Infof("querying last kline exchange = %s AND symbol = %s AND interval = %s", ex, symbol, interval)

	// make the SQL syntax IDE friendly, so that it can analyze it.
	sql := "SELECT * FROM binance_klines WHERE  `symbol` = :symbol AND `interval` = :interval ORDER BY end_time DESC LIMIT 1"
	sql = strings.ReplaceAll(sql, "binance_klines", ex.String()+"_klines")

	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"exchange": ex,
		"interval": interval,
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
		var kline types.KLine
		err = rows.StructScan(&kline)
		return &kline, err
	}

	return nil, rows.Err()
}

func (s *BacktestService) QueryKLinesForward(exchange types.ExchangeName, symbol string, interval types.Interval, startTime time.Time, limit int) ([]types.KLine, error) {
	sql := "SELECT * FROM `binance_klines` WHERE `end_time` >= :start_time AND `symbol` = :symbol AND `interval` = :interval ORDER BY end_time ASC LIMIT :limit"
	sql = strings.ReplaceAll(sql, "binance_klines", exchange.String()+"_klines")

	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"start_time": startTime,
		"limit": limit,
		"symbol":    symbol,
		"interval":  interval,
	})
	if err != nil {
		return nil, err
	}

	return s.scanRows(rows)
}

func (s *BacktestService) QueryKLinesBackward(exchange types.ExchangeName, symbol string, interval types.Interval, endTime time.Time, limit int) ([]types.KLine, error) {
	sql := "SELECT * FROM `binance_klines` WHERE `end_time` <= :end_time AND `symbol` = :symbol AND `interval` = :interval ORDER BY end_time ASC LIMIT :limit"
	sql = strings.ReplaceAll(sql, "binance_klines", exchange.String()+"_klines")

	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"limit":    limit,
		"end_time":  endTime,
		"symbol":   symbol,
		"interval": interval,
	})
	if err != nil {
		return nil, err
	}

	return s.scanRows(rows)
}

func (s *BacktestService) QueryKLinesCh(since, until time.Time, exchange types.Exchange, symbols []string, intervals []types.Interval) (chan types.KLine, chan error) {
	sql := "SELECT * FROM `binance_klines` WHERE `end_time` BETWEEN :since AND :until AND `symbol` IN (:symbols) AND `interval` IN (:intervals) ORDER BY end_time ASC"
	sql = strings.ReplaceAll(sql, "binance_klines", exchange.Name().String()+"_klines")

	sql, args, err := sqlx.Named(sql, map[string]interface{}{
		"since":     since,
		"until":     until,
		"symbols":   symbols,
		"intervals": types.IntervalSlice(intervals),
	})

	sql, args, err = sqlx.In(sql, args...)
	sql = s.DB.Rebind(sql)

	rows, err := s.DB.Queryx(sql, args...)
	if err != nil {
		log.WithError(err).Error("query error")

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

func (s *BacktestService) Insert(kline types.KLine) error {
	if len(kline.Exchange) == 0 {
		return errors.New("kline.Exchange field should not be empty")
	}

	sql := "INSERT INTO `binance_klines` (`exchange`, `start_time`, `end_time`, `symbol`, `interval`, `open`, `high`, `low`, `close`, `closed`, `volume`, `quote_volume`, `taker_buy_base_volume`, `taker_buy_quote_volume`)" +
		"VALUES (:exchange, :start_time, :end_time, :symbol, :interval, :open, :high, :low, :close, :closed, :volume, :quote_volume, :taker_buy_base_volume, :taker_buy_quote_volume)"
	sql = strings.ReplaceAll(sql, "binance_klines", kline.Exchange.String()+"_klines")

	_, err := s.DB.NamedExec(sql, kline)
	return err
}
