package service

import (
	"context"
	"fmt"
	"os"
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
	log.Debugf("inserted klines %s %s data: %d", symbol, interval.String(), count)

	if err := <-errC; err != nil {
		return err
	}

	return nil
}

func (s *BacktestService) Verify(symbols []string, startTime time.Time, endTime time.Time, sourceExchange types.Exchange, verboseCnt int) error {
	var corruptCnt = 0
	for _, symbol := range symbols {
		log.Infof("verifying backtesting data...")

		for interval := range types.SupportedIntervals {
			log.Infof("verifying %s %s kline data...", symbol, interval)

			klineC, errC := s.QueryKLinesCh(startTime, endTime, sourceExchange, []string{symbol}, []types.Interval{interval})
			var emptyKLine types.KLine
			var prevKLine types.KLine
			for k := range klineC {
				if verboseCnt > 1 {
					fmt.Fprint(os.Stderr, ".")
				}

				if prevKLine != emptyKLine {
					if prevKLine.StartTime.Unix() == k.StartTime.Unix() {
						s._deleteDuplicatedKLine(k)
						log.Errorf("found kline data duplicated at time: %s kline: %+v , deleted it", k.StartTime, k)
					} else if prevKLine.StartTime.Time().Add(interval.Duration()).Unix() != k.StartTime.Time().Unix() {
						corruptCnt++
						log.Errorf("found kline data corrupted at time: %s kline: %+v", k.StartTime, k)
						log.Errorf("between %d and %d",
							prevKLine.StartTime.Unix(),
							k.StartTime.Unix())
					}
				}

				prevKLine = k
			}

			if verboseCnt > 1 {
				fmt.Fprintln(os.Stderr)
			}

			if err := <-errC; err != nil {
				return err
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

func (s *BacktestService) Sync(ctx context.Context, exchange types.Exchange, symbol string,
	startTime time.Time, endTime time.Time, interval types.Interval) error {

	return s.SyncKLineByInterval(ctx, exchange, symbol, interval, startTime, endTime)

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
		return returnError(errors.Errorf("symbols is empty when querying kline, plesae check your strategy setting. "))
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
	if err != nil {
		return returnError(err)
	}
	sql = s.DB.Rebind(sql)

	rows, err := s.DB.Queryx(sql, args...)
	if err != nil {
		return returnError(err)
	}

	return s.scanRowsCh(rows)
}

func returnError(err error) (chan types.KLine, chan error) {
	ch := make(chan types.KLine, 0)
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
	case types.ExchangeKucoin:
		return "kucoin_klines"
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

	tx := s.DB.MustBegin()
	if _, err := tx.NamedExec(sql, kline); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *BacktestService) _deleteDuplicatedKLine(k types.KLine) error {

	if len(k.Exchange) == 0 {
		return errors.New("kline.Exchange field should not be empty")
	}

	tableName := s._targetKlineTable(k.Exchange)
	sql := fmt.Sprintf("delete from `%s` where gid = :gid  ", tableName)
	_, err := s.DB.NamedExec(sql, k)
	return err
}

func (s *BacktestService) SyncExist(ctx context.Context, exchange types.Exchange, symbol string,
	fromTime time.Time, endTime time.Time, interval types.Interval) error {
	klineC, errC := s.QueryKLinesCh(fromTime, endTime, exchange, []string{symbol}, []types.Interval{interval})

	nowStartTime := fromTime
	for k := range klineC {
		if nowStartTime.Unix() < k.StartTime.Unix() {
			log.Infof("syncing %s interval %s syncing %s ~ %s ", symbol, interval, nowStartTime, k.EndTime)
			if err := s.Sync(ctx, exchange, symbol, nowStartTime, k.EndTime.Time().Add(-1*interval.Duration()), interval); err != nil {
				log.WithError(err).Errorf("sync error")
			}
		}
		nowStartTime = k.StartTime.Time().Add(interval.Duration())
	}

	if nowStartTime.Unix() < endTime.Unix() && nowStartTime.Unix() < time.Now().Unix() {
		if err := s.Sync(ctx, exchange, symbol, nowStartTime, endTime, interval); err != nil {
			log.WithError(err).Errorf("sync error")
		}
	}

	if err := <-errC; err != nil {
		return err
	}
	return nil
}
