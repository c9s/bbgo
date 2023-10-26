package csvsource

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// KlineReader is an interface for reading candlesticks.
type KlineReader interface {
	Read(interval time.Duration) (types.KLine, error)
	ReadAll(interval time.Duration) ([]types.KLine, error)
}

// ReadKlinesFromCSV reads all the .csv files in a given directory or a single file into a slice of Klines.
// Wraps a default CSVKlineReader with Binance decoder for convenience.
// For finer grained memory management use the base kline reader.
func ReadKlinesFromCSV(path string, interval time.Duration) ([]types.KLine, error) {
	return ReadKlinesFromCSVWithDecoder(path, interval, MakeCSVKlineReader(NewBinanceCSVKlineReader))
}

// ReadKlinesFromCSVWithDecoder permits using a custom CSVKlineReader.
func ReadKlinesFromCSVWithDecoder(path string, interval time.Duration, maker MakeCSVKlineReader) ([]types.KLine, error) {
	var prices []types.KLine

	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
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
		klines, err := reader.ReadAll(interval)
		if err != nil {
			return err
		}
		prices = append(prices, klines...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return prices, nil
}
