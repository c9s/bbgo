package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/datasource/csvsource"
	"github.com/c9s/bbgo/pkg/types"
)

type BacktestServiceCSV struct {
	kLines      map[types.Interval][]types.KLine
	path        string
	market      csvsource.MarketType
	granularity csvsource.DataType
}

func NewBacktestServiceCSV(
	path string,
	market csvsource.MarketType,
	granularity csvsource.DataType,
) BackTestable {
	return &BacktestServiceCSV{
		kLines:      make(map[types.Interval][]types.KLine),
		path:        path,
		market:      market,
		granularity: granularity,
	}
}

func (s *BacktestServiceCSV) Verify(sourceExchange types.Exchange, symbols []string, startTime time.Time, endTime time.Time) error {
	// TODO: verify the content of the csv file
	// check if the csv file exists
	if _, err := os.Open(s.path); err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}

	return nil
}

func (s *BacktestServiceCSV) Sync(ctx context.Context, exchange types.Exchange, symbol string, intervals []types.Interval, startTime, endTime time.Time) error {

	log.Infof("starting fresh csv sync %s %s: %s <=> %s", exchange.Name(), symbol, startTime, endTime)

	path := fmt.Sprintf("%s/%s/%s", s.path, exchange.Name().String(), symbol)

	kLineMap, err := csvsource.ReadTicksFromCSV(
		path,
		symbol,
		intervals,
	)
	if err != nil {
		return errors.Errorf("reading csv data: %v", err)
	}

	s.kLines = kLineMap

	return nil
}

// QueryKLine queries the klines from the database
func (s *BacktestServiceCSV) QueryKLine(ex types.Exchange, symbol string, interval types.Interval, orderBy string, limit int) (*types.KLine, error) {
	log.Infof("querying last kline exchange = %s AND symbol = %s AND interval = %s", ex.Name(), symbol, interval)
	if _, ok := s.kLines[interval]; !ok || len(s.kLines[interval]) == 0 {
		return nil, errors.New("interval not initialized")
	}
	return &s.kLines[interval][len(s.kLines[interval])-1], nil
}

// QueryKLinesForward is used for querying klines to back-testing
func (s *BacktestServiceCSV) QueryKLinesForward(exchange types.Exchange, symbol string, interval types.Interval, startTime time.Time, limit int) ([]types.KLine, error) {
	// Sample implementation (modify as needed):
	var result []types.KLine

	// Access klines data based on exchange, symbol, and interval
	exchangeKLines, ok := s.kLines[interval]
	if !ok {
		return nil, fmt.Errorf("no kLines for specified interval %s", interval.String())
	}

	// Filter klines based on startTime and limit
	for _, kline := range exchangeKLines {
		if kline.StartTime.After(startTime) {
			result = append(result, kline)
			if len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}

func (s *BacktestServiceCSV) QueryKLinesBackward(exchange types.Exchange, symbol string, interval types.Interval, endTime time.Time, limit int) ([]types.KLine, error) {
	var result []types.KLine

	// Access klines data based on interval
	exchangeKLines, ok := s.kLines[interval]
	if !ok {
		return nil, fmt.Errorf("no kLines for specified interval %s", interval.String())
	}

	// Reverse iteration through klines and filter based on endTime and limit
	for i := len(exchangeKLines) - 1; i >= 0; i-- {
		kline := exchangeKLines[i]

		if kline.StartTime.Before(endTime) {
			result = append(result, kline)
			if len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}

func (s *BacktestServiceCSV) QueryKLinesCh(since, until time.Time, exchange types.Exchange, symbols []string, intervals []types.Interval) (chan types.KLine, chan error) {
	if len(symbols) == 0 {
		return returnError(errors.Errorf("symbols is empty when querying kline, please check your strategy setting. "))
	}

	ch := make(chan types.KLine, len(s.kLines))
	go func() {
		defer close(ch)
		for _, kline := range s.kLines[intervals[0]] {
			ch <- kline
		}
	}()

	return ch, nil
}
