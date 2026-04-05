package csvsource

import (
	"encoding/csv"
	"io"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

var _ KLineReader = (*CSVKLineReader)(nil)

// CSVKLineReader is a KLineReader that reads from a CSV file.
type CSVKLineReader struct {
	csv *csv.Reader
}

// NewCSVKLineReader creates a new CSVKLineReader with the default Binance decoder.
func NewCSVKLineReader(csv *csv.Reader) *CSVKLineReader {
	return &CSVKLineReader{
		csv: csv,
	}
}

// Read reads the next KLine from the underlying CSV data.
func (r *CSVKLineReader) Read(interval time.Duration) (types.KLine, error) {
	var k types.KLine

	rec, err := r.csv.Read()
	if err != nil {
		return k, err
	}

	return parseCsvKLineRecord(rec, interval)
}

// ReadAll reads all the KLines from the underlying CSV data.
func (r *CSVKLineReader) ReadAll(interval time.Duration) ([]types.KLine, error) {
	var ks []types.KLine
	for {
		k, err := r.Read(interval)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ks = append(ks, k)
	}

	return ks, nil
}
