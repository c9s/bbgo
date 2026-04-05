package csvsource

import (
	"encoding/csv"
	"io"
)

var _ TickReader = (*CSVTickReader)(nil)

// CSVTickReader reads CsvTick records from a CSV file.
type CSVTickReader struct {
	csv *csv.Reader
}

// NewCSVTickReader creates a new CSVTickReader.
func NewCSVTickReader(csv *csv.Reader) *CSVTickReader {
	return &CSVTickReader{
		csv: csv,
	}
}

// ReadAll reads all CsvTicks from the underlying CSV data.
func (r *CSVTickReader) ReadAll() (ticks []*CsvTick, err error) {
	for {
		tick, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if tick == nil {
			continue
		}

		ticks = append(ticks, tick)
	}

	return ticks, nil
}

// Read reads the next CsvTick from the underlying CSV data.
func (r *CSVTickReader) Read() (*CsvTick, error) {
	rec, err := r.csv.Read()
	if err != nil {
		return nil, err
	}

	return CSVTickDecoder(rec)
}
