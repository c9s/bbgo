package csvsource

import (
	"encoding/csv"
	"io"

	"github.com/c9s/bbgo/pkg/types"
)

var _ TickReader = (*CSVTickReader)(nil)

// CSVTickReader is a CSVTickReader that reads from a CSV file.
type CSVTickReader struct {
	csv     *csv.Reader
	decoder CSVTickDecoder
}

// MakeCSVTickReader is a factory method type that creates a new CSVTickReader.
type MakeCSVTickReader func(csv *csv.Reader) *CSVTickReader

// NewCSVKLineReader creates a new CSVKLineReader with the default Binance decoder.
func NewCSVTickReader(csv *csv.Reader) *CSVTickReader {
	return &CSVTickReader{
		csv:     csv,
		decoder: BinanceCSVTickDecoder,
	}
}

// NewCSVTickReaderWithDecoder creates a new CSVKLineReader with the given decoder.
func NewCSVTickReaderWithDecoder(csv *csv.Reader, decoder CSVTickDecoder) *CSVTickReader {
	return &CSVTickReader{
		csv:     csv,
		decoder: decoder,
	}
}

// ReadAll reads all the KLines from the underlying CSV data.
func (r *CSVTickReader) ReadAll(interval types.Interval) error {
	var i int
	for {
		tick, err := r.Read(i)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		i++
		if tick == nil {
			continue
		}
		ConvertCsvTickToKLines(tick, interval)
	}

	return nil
}

// Read reads the next KLine from the underlying CSV data.
func (r *CSVTickReader) Read(i int) (*CsvTick, error) {
	rec, err := r.csv.Read()
	if err != nil {
		return nil, err
	}

	return r.decoder(rec, i)
}
