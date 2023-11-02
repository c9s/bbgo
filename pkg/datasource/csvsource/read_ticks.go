package csvsource

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/c9s/bbgo/pkg/types"
)

// TickReader is an interface for reading candlesticks.
type TickReader interface {
	Read(i int) (*CsvTick, error)
	ReadAll() (ticks []*CsvTick, err error)
}

// ReadTicksFromCSV reads all the .csv files in a given directory or a single file into a slice of Ticks.
// Wraps a default CSVTickReader with Binance decoder for convenience.
// For finer grained memory management use the base kline reader.
func ReadTicksFromCSV(
	path, symbol string,
	intervals []types.Interval,
) (
	klineMap map[types.Interval][]types.KLine,
	err error,
) {
	return ReadTicksFromCSVWithDecoder(
		path,
		symbol,
		intervals,
		MakeCSVTickReader(NewBinanceCSVTickReader),
	)
}

// ReadTicksFromCSVWithDecoder permits using a custom CSVTickReader.
func ReadTicksFromCSVWithDecoder(
	path, symbol string,
	intervals []types.Interval,
	maker MakeCSVTickReader,
) (
	klineMap map[types.Interval][]types.KLine,
	err error,
) {
	converter := NewCSVTickConverter(intervals)
	ticks := []*CsvTick{}
	// read all ticks into memory
	err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".csv" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		//nolint:errcheck // Read ops only so safe to ignore err return
		defer file.Close()
		reader := maker(csv.NewReader(file))
		newTicks, err := reader.ReadAll()
		if err != nil {
			return err
		}
		ticks = append(ticks, newTicks...)

		return nil
	})

	if err != nil {
		return nil, err
	}
	// sort ticks by timestamp (okex sorts csv by price ascending ;(
	sort.Slice(ticks, func(i, j int) bool {
		return ticks[i].Timestamp.Time().Before(ticks[j].Timestamp.Time())
	})

	for _, tick := range ticks {
		tick.Symbol = symbol
		converter.CsvTickToKLine(tick)
	}

	return converter.GetKLineResults(), nil
}
