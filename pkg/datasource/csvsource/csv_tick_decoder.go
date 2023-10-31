package csvsource

import (
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// CSVTickDecoder is an extension point for CSVTickReader to support custom file formats.
type CSVTickDecoder func(record []string, index int) (*CsvTick, error)

// NewBinanceCSVTickReader creates a new CSVTickReader for Binance CSV files.
func NewBinanceCSVTickReader(csv *csv.Reader) *CSVTickReader {
	return &CSVTickReader{
		csv:     csv,
		decoder: BinanceCSVTickDecoder,
	}
}

// BinanceCSVKLineDecoder decodes a CSV record from Binance into a CsvTick.
func BinanceCSVTickDecoder(row []string, intex int) (*CsvTick, error) {
	if len(row) < 7 {
		return nil, ErrNotEnoughColumns
	}
	timestamp, err := strconv.ParseInt(row[5], 10, 64)
	if err != nil {
		return nil, ErrInvalidTimeFormat
	}
	size, err := strconv.ParseFloat(row[2], 64)
	if err != nil {
		return nil, ErrInvalidVolumeFormat
	}
	price, err := strconv.ParseFloat(row[1], 64)
	if err != nil {
		return nil, ErrInvalidPriceFormat
	}
	return &CsvTick{
		Timestamp:    timestamp / 1000,
		Size:         fixedpoint.NewFromFloat(size),
		Price:        fixedpoint.NewFromFloat(price),
		HomeNotional: fixedpoint.NewFromFloat(price * size),
	}, nil
}

// NewBinanceCSVTickReader creates a new CSVTickReader for Bybit CSV files.
func NewBybitCSVTickReader(csv *csv.Reader) *CSVTickReader {
	return &CSVTickReader{
		csv:     csv,
		decoder: BybitCSVTickDecoder,
	}
}

// BybitCSVTickDecoder decodes a CSV record from Bybit into a CsvTick.
func BybitCSVTickDecoder(row []string, index int) (*CsvTick, error) {
	if len(row) < 9 {
		return nil, ErrNotEnoughColumns
	}
	if index == 0 {
		return nil, nil
	}
	startTime := strings.Split(row[0], ".")[0]
	timestamp, err := strconv.ParseInt(startTime, 10, 64)
	if err != nil {
		return nil, ErrInvalidTimeFormat
	}
	size, err := strconv.ParseFloat(row[3], 64)
	if err != nil {
		return nil, ErrInvalidVolumeFormat
	}
	price, err := strconv.ParseFloat(row[4], 64)
	if err != nil {
		return nil, ErrInvalidPriceFormat
	}
	homeNotional, err := strconv.ParseFloat(row[8], 64)
	if err != nil {
		return nil, ErrInvalidVolumeFormat
	}
	foreignNotional, err := strconv.ParseFloat(row[9], 64)
	if err != nil {
		return nil, ErrInvalidVolumeFormat
	}

	return &CsvTick{
		Timestamp:       timestamp,
		Symbol:          row[1],
		Side:            row[2],
		Size:            fixedpoint.NewFromFloat(size),
		Price:           fixedpoint.NewFromFloat(price),
		TickDirection:   row[5],
		HomeNotional:    fixedpoint.NewFromFloat(homeNotional),
		ForeignNotional: fixedpoint.NewFromFloat(foreignNotional),
	}, nil
}
