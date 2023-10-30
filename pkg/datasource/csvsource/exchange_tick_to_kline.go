package csvsource

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var klines []types.KLine

type BybitCsvTick struct {
	Timestamp       int64            `json:"timestamp"`
	Symbol          string           `json:"symbol"`
	Side            string           `json:"side"`
	TickDirection   string           `json:"tickDirection"`
	Size            fixedpoint.Value `json:"size"`
	Price           fixedpoint.Value `json:"price"`
	HomeNotional    fixedpoint.Value `json:"homeNotional"`
	ForeignNotional fixedpoint.Value `json:"foreignNotional"`
}

func ConvertTicksToKLines(path, symbol string, interval KLineInterval) ([]types.KLine, error) {
	err := filepath.Walk(
		path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("walk %s: %w", path, err)
			}
			if !info.IsDir() {
				file, err := os.Open(path)
				if err != nil {
					return fmt.Errorf("open %s: %w", path, err)
				}
				reader := csv.NewReader(file)
				data, err := reader.ReadAll()
				if err != nil {
					return fmt.Errorf("read %s: %w", path, err)
				}
				for idx, row := range data {
					// skip header
					if idx == 0 {
						continue
					}
					if idx == 1 {
						continue
					}
					startTime := strings.Split(row[0], ".")[0]
					timestamp, err := strconv.ParseInt(startTime, 10, 64)
					if err != nil {
						return fmt.Errorf("parse time %s: %w", startTime, err)
					}
					size, err := strconv.ParseFloat(row[3], 64)
					if err != nil {
						return fmt.Errorf("parse size %s: %w", row[3], err)
					}
					price, err := strconv.ParseFloat(row[4], 64)
					if err != nil {
						return fmt.Errorf("parse price %s: %w", row[4], err)
					}
					homeNotional, err := strconv.ParseFloat(row[8], 64)
					if err != nil {
						return fmt.Errorf("parse home notional %s: %w", row[8], err)
					}
					foreignNotional, err := strconv.ParseFloat(row[9], 64)
					if err != nil {
						return fmt.Errorf("parse foreign notional %s: %w", row[9], err)
					}
					ConvertBybitCsvTickToCandles(BybitCsvTick{
						Timestamp:       int64(timestamp),
						Symbol:          row[1],
						Side:            row[2],
						Size:            fixedpoint.NewFromFloat(size),
						Price:           fixedpoint.NewFromFloat(price),
						TickDirection:   row[5],
						HomeNotional:    fixedpoint.NewFromFloat(homeNotional),
						ForeignNotional: fixedpoint.NewFromFloat(foreignNotional),
					}, interval)
				}
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	return klines, nil
}

// Conver ticks to KLine with interval
func ConvertBybitCsvTickToCandles(tick BybitCsvTick, interval KLineInterval) {
	var (
		currentCandle = types.KLine{}
		high          = fixedpoint.Zero
		low           = fixedpoint.Zero
		tickTimeStamp = time.Unix(tick.Timestamp, 0)
	)
	isOpen, isCLose, openTime := detCandleStart(tickTimeStamp, interval)

	if isOpen {
		klines = append(klines, types.KLine{
			StartTime: types.NewTimeFromUnix(openTime.Unix(), 0),
			Open:      tick.Price,
			High:      tick.Price,
			Low:       tick.Price,
			Close:     tick.Price,
			Volume:    tick.HomeNotional,
		})
		return
	}

	currentCandle = klines[len(klines)-1]

	if tick.Price > currentCandle.High {
		high = tick.Price
	} else {
		high = currentCandle.High
	}

	if tick.Price < currentCandle.Low {
		low = tick.Price
	} else {
		low = currentCandle.Low
	}

	kline := types.KLine{
		StartTime: currentCandle.StartTime,
		Open:      currentCandle.Open,
		High:      high,
		Low:       low,
		Close:     tick.Price,
		Volume:    currentCandle.Volume.Add(tick.HomeNotional),
	}
	if isCLose {
		klines = append(klines, kline)
	} else {
		klines[len(klines)-1] = kline
	}
}

// WriteKLines writes csv to path.
func WriteKLines(path string, prices []types.KLine) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "failed to open file")
	}
	defer func() {
		err = file.Close()
		if err != nil {
			panic("failed to close file")
		}
	}()
	w := csv.NewWriter(file)
	defer w.Flush()
	// Using Write
	for _, record := range prices {
		row := []string{strconv.Itoa(int(record.StartTime.UnixMilli())), record.Open.String(), record.High.String(), record.Low.String(), record.Close.String(), record.Volume.String()}
		if err := w.Write(row); err != nil {
			return errors.Wrap(err, "writing record to file")
		}
	}
	if err != nil {
		return err
	}

	return nil
}

func detCandleStart(ts time.Time, kInterval KLineInterval) (isOpen, isClose bool, t time.Time) {
	interval := convertInterval(kInterval)
	if len(klines) == 0 {
		start := time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour()+1, 0, 0, 0, ts.Location())
		if t.Minute() < int(interval.Minutes()) { // todo make enum of supported intervals 5 10 15 30 and extend to other intervals
			start = time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), int(interval.Minutes()), 0, 0, ts.Location())
		}
		return true, false, start
	} else {
		current := klines[len(klines)-1]
		end := current.StartTime.Time().Add(interval)
		if end.After(ts) {
			return false, true, end
		}
	}

	return false, false, t
}

func convertInterval(kInterval KLineInterval) time.Duration {
	var interval = time.Minute
	switch kInterval {
	case M1:
	case M5:
		interval = time.Minute * 5
	case M15:
		interval = time.Minute * 15
	case M30:
		interval = time.Minute * 30
	case H1:
		interval = time.Hour
	case H2:
		interval = time.Hour * 2
	case H4:
		interval = time.Hour * 4
	case D1:
		interval = time.Hour * 24
	}
	return interval
}
