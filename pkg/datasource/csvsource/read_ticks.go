package csvsource

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"

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
func ReadTicksFromCSV(path, symbol string, interval types.Interval) ([]types.KLine, error) {
	return ReadTicksFromCSVWithDecoder(path, symbol, interval, MakeCSVTickReader(NewBinanceCSVTickReader))
}

// ReadTicksFromCSVWithDecoder permits using a custom CSVTickReader.
func ReadTicksFromCSVWithDecoder(path string, symbol string, interval types.Interval, maker MakeCSVTickReader) (klines []types.KLine, err error) {
	converter := NewCSVTickConverter()

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
		reader := maker(csv.NewReader(file)) // todo this is wrong need to pass instantiated converter
		newTicks, err := reader.ReadAll()
		if err != nil {
			return err
		}
		for _, tick := range newTicks {
			tick.Symbol = symbol
			converter.CsvTickToKLine(tick, interval)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return converter.GetKLineResult(), nil
}
