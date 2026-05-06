package csvsource

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

// KLineReader is an interface for reading candlesticks.
type KLineReader interface {
	Read() (types.KLine, error)
	ReadAll() ([]types.KLine, error)
}

// ReadAllKLineCsv reads all the .csv files in a given directory or a single file into a slice of KLines.
// Wraps a default CSVKLineReader with Binance decoder for convenience.
// For finer grained memory management use the base kline reader.
func ReadAllKLineCsv(dir string, symbol string, interval types.Interval) ([]types.KLine, error) {
	var klines []types.KLine

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
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

		log.Infof("reading %s", file.Name())
		reader := NewCSVKLineReader(csv.NewReader(file), symbol, interval)
		fileKLines, err := reader.ReadAll()
		if err != nil {
			return err
		}

		klines = append(klines, fileKLines...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return klines, nil
}
