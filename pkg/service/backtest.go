package service

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

type BacktestService struct {
	DB *sqlx.DB
}

func (s *BacktestService) Sync(ctx context.Context, exchange types.Exchange, symbol string, startTime time.Time) error {
	lastKLine, err := s.QueryLast(exchange.Name(), symbol, "1m")
	if err != nil {
		return err
	}

	if lastKLine != nil {
		startTime = lastKLine.EndTime
	}

	for interval := range types.SupportedIntervals {
		log.Infof("synchronizing lastKLine for interval %s from exchange %s", interval, exchange.Name())
		batch := &types.ExchangeBatchProcessor{Exchange: exchange}

		// should use channel here
		allKLines, err := batch.BatchQueryKLines(ctx, symbol, interval, startTime, time.Now())
		if err != nil {
			return err
		}

		for _, k := range allKLines {
			if err := s.Insert(k); err != nil {
				return err
			}
		}
	}

	return nil
}

// QueryLast queries the last order from the database
func (s *BacktestService) QueryLast(ex types.ExchangeName, symbol, interval string) (*types.KLine, error) {
	log.Infof("querying last kline exchange = %s AND symbol = %s AND interval = %s", ex, symbol, interval)

	// make the SQL syntax IDE friendly, so that it can analyze it.
	sql := "SELECT * FROM binance_klines WHERE  `symbol` = :symbol AND `interval` = :interval ORDER BY gid DESC LIMIT 1"

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

func (s *BacktestService) QueryKLinesCh(since time.Time, ex types.ExchangeName, symbol string, intervals ...string) (chan types.KLine, error) {
	sql := "SELECT * FROM `binance_klines` WHERE `end_time` >= :since AND `symbol` = :symbol AND `interval` IN (:intervals) ORDER BY end_time ASC"
	sql = strings.ReplaceAll(sql, "binance_klines", ex.String()+"_klines")

	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"since":     since,
		"exchange":  ex,
		"symbol":    symbol,
		"intervals": intervals,
	})
	if err != nil {
		return nil, err
	}

	return s.scanRowsCh(rows), nil
}

// scanRowsCh scan rows into channel
func (s *BacktestService) scanRowsCh(rows *sqlx.Rows) chan types.KLine {
	ch := make(chan types.KLine, 100)

	go func() {
		defer rows.Close()

		for rows.Next() {
			var kline types.KLine
			if err := rows.StructScan(&kline); err != nil {
				log.WithError(err).Error("kline scan error")
				continue
			}

			ch <- kline
		}

		if err := rows.Err(); err != nil {
			log.WithError(err).Error("kline scan error")
		}
	}()

	return ch
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

	sql := "INSERT INTO `binance_klines` (`start_time`, `end_time`, `symbol`, `interval`, `open`, `high`, `low`, `close`, `closed`, `volume`)" +
		"VALUES (:start_time, :end_time, :symbol, :interval, :open, :high, :low, :close, :closed, :volume)"
	sql = strings.ReplaceAll(sql, "binance_klines", kline.Exchange+"_klines")

	_, err := s.DB.NamedExec(sql, kline)
	return err
}
