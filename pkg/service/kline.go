package service

import (
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

type KLineService struct {
	DB *sqlx.DB
}

// QueryLast queries the last order from the database
func (s *KLineService) QueryLast(ex types.ExchangeName, symbol, interval string) (*types.KLine, error) {
	log.Infof("querying last kline exchange = %s AND symbol = %s", ex, symbol)

	// make the SQL syntax IDE friendly, so that it can analyze it.
	table := ex.String() + "_klines"
	sql := "SELECT * FROM binance_klines WHERE  `symbol` = :symbol AND `interval` = :interval ORDER BY gid DESC LIMIT 1"
	sql = strings.ReplaceAll(sql, "binance_klines", table)

	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"table":    table,
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
		return &order, err
	}

	return nil, rows.Err()
}

func (s *KLineService) Query(ex types.ExchangeName, symbol string) ([]types.KLine, error) {
	rows, err := s.DB.NamedQuery(`SELECT * FROM orders WHERE exchange = :exchange AND symbol = :symbol ORDER BY gid ASC`, map[string]interface{}{
		"exchange": ex,
		"symbol":   symbol,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
}

func (s *KLineService) scanRows(rows *sqlx.Rows) (klines []types.KLine, err error) {
	for rows.Next() {
		var kline types.KLine
		if err := rows.StructScan(&kline); err != nil {
			return nil, err
		}

		klines = append(klines, kline)
	}

	return klines, rows.Err()
}

func (s *KLineService) Insert(kline types.KLine) error {
	table := kline.Exchange + "_klines"
	sql := `INSERT INTO binance_klines (start_time, end_time, symbol, interval, open, high, low, close, closed, volume)
	VALUES (:start_time, :end_time, :symbol, :interval, :open, :high, :low, :close, :closed, :volume)`

	sql = strings.ReplaceAll(sql, "binance_klines", table)
	_, err := s.DB.NamedExec(sql, kline)
	return err
}
