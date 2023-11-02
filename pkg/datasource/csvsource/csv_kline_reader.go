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
	csv     *csv.Reader
	decoder CSVKLineDecoder
}

// MakeCSVKLineReader is a factory method type that creates a new CSVKLineReader.
type MakeCSVKLineReader func(csv *csv.Reader) *CSVKLineReader

// NewCSVKLineReader creates a new CSVKLineReader with the default Binance decoder.
func NewCSVKLineReader(csv *csv.Reader) *CSVKLineReader {
	return &CSVKLineReader{
		csv:     csv,
		decoder: BinanceCSVKLineDecoder,
	}
}

// NewCSVKLineReaderWithDecoder creates a new CSVKLineReader with the given decoder.
func NewCSVKLineReaderWithDecoder(csv *csv.Reader, decoder CSVKLineDecoder) *CSVKLineReader {
	return &CSVKLineReader{
		csv:     csv,
		decoder: decoder,
	}
}

// Read reads the next KLine from the underlying CSV data.
func (r *CSVKLineReader) Read(interval time.Duration) (types.KLine, error) {
	var k types.KLine

	rec, err := r.csv.Read()
	if err != nil {
		return k, err
	}

	return r.decoder(rec, interval)
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
