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

func ConvertTicksToKLines(symbol string, interval time.Duration) error {
	err := filepath.Walk(
		fmt.Sprintf("pkg/datasource/csv/testdata/bybit/%s/", symbol),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Println(err)
				return err
			}
			fmt.Printf("dir: %v: name: %s\n", info.IsDir(), path)
			if !info.IsDir() {
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				reader := csv.NewReader(file)
				data, err := reader.ReadAll()
				if err != nil {
					return err
				}
				for idx, row := range data {
					// skip header
					if idx == 0 {
						continue
					}
					if idx == 1 {
						continue
					}
					timestamp, err := strconv.ParseInt(strings.Split(row[0], ".")[0], 10, 64)
					if err != nil {
						return err
					}
					size, err := strconv.ParseFloat(row[3], 64)
					if err != nil {
						return err
					}
					price, err := strconv.ParseFloat(row[4], 64)
					if err != nil {
						return err
					}
					homeNotional, err := strconv.ParseFloat(row[5], 64)
					if err != nil {
						return err
					}
					foreignNotional, err := strconv.ParseFloat(row[9], 64)
					if err != nil {
						return err
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
		return err
	}

	return WriteKLines(fmt.Sprintf("pkg/datasource/csv/testdata/%s_%s.csv", symbol, interval.String()), klines)
}

// WriteKLines write csv to path.
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
		row := []string{strconv.Itoa(int(record.StartTime.UnixMilli())), dtos(record.Open), dtos(record.High), dtos(record.Low), dtos(record.Close), dtos(record.Volume)}
		if err := w.Write(row); err != nil {
			return errors.Wrap(err, "writing record to file")
		}
	}
	if err != nil {
		return err
	}

	return nil
}

func dtos(n fixedpoint.Value) string {
	return fmt.Sprintf("%f", n.Float64())
}

func ConvertBybitCsvTickToCandles(tick BybitCsvTick, interval time.Duration) {
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

func detCandleStart(ts time.Time, interval time.Duration) (isOpen, isClose bool, t time.Time) {
	if len(klines) == 0 {
		start := time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour()+1, 0, 0, 0, ts.Location())
		if t.Minute() < int(interval.Minutes()) { // supported intervals 5 10 15 30
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
