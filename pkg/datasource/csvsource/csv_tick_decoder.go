package csvsource

import (
	"encoding/csv"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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
func BinanceCSVTickDecoder(row []string, _ int) (*CsvTick, error) {
	if len(row) < 7 {
		return nil, ErrNotEnoughColumns
	}
	size := fixedpoint.MustNewFromString(row[2])
	price := fixedpoint.MustNewFromString(row[1])
	hn := price.Mul(size)
	return &CsvTick{
		Timestamp:    types.MustParseMillisecondTimestamp(row[5]),
		Size:         size,
		Price:        price,
		HomeNotional: hn,
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
	side, err := types.StrToSideType(row[2])
	if err != nil {
		return nil, ErrInvalidOrderSideFormat
	}
	return &CsvTick{
		Timestamp:       types.MustParseMillisecondTimestamp(row[0]),
		Symbol:          row[1],
		Side:            side,
		TickDirection:   row[5],
		Size:            fixedpoint.MustNewFromString(row[3]),
		Price:           fixedpoint.MustNewFromString(row[4]),
		HomeNotional:    fixedpoint.MustNewFromString(row[8]),
		ForeignNotional: fixedpoint.MustNewFromString(row[9]),
	}, nil
}
