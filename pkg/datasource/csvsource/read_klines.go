package csvsource

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// KLineReader is an interface for reading candlesticks.
type KLineReader interface {
	Read(interval time.Duration) (types.KLine, error)
	ReadAll(interval time.Duration) ([]types.KLine, error)
}

// ReadKLinesFromCSV reads all the .csv files in a given directory or a single file into a slice of KLines.
// Wraps a default CSVKLineReader with Binance decoder for convenience.
// For finer grained memory management use the base kline reader.
func ReadKLinesFromCSV(path string, interval time.Duration) ([]types.KLine, error) {
	return ReadKLinesFromCSVWithDecoder(path, interval, MakeCSVKLineReader(NewBinanceCSVKLineReader))
}

// ReadKLinesFromCSVWithDecoder permits using a custom CSVKLineReader.
func ReadKLinesFromCSVWithDecoder(path string, interval time.Duration, maker MakeCSVKLineReader) ([]types.KLine, error) {
	var klines []types.KLine

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
		newKlines, err := reader.ReadAll(interval)
		if err != nil {
			return err
		}
		klines = append(klines, newKlines...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return klines, nil
}
