package csvsource

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// MetaTraderTimeFormat is the time format expected by the MetaTrader decoder when cols [0] and [1] are used.
const MetaTraderTimeFormat = "02/01/2006 15:04"

var (
	// ErrNotEnoughColumns is returned when the CSV price record does not have enough columns.
	ErrNotEnoughColumns = errors.New("not enough columns")

	// ErrInvalidTimeFormat is returned when the CSV price record does not have a valid time unix milli format.
	ErrInvalidIDFormat = errors.New("cannot parse trade id string")

	// ErrInvalidBoolFormat is returned when the CSV isBuyerMaker record does not have a valid bool representation.
	ErrInvalidBoolFormat = errors.New("cannot parse bool to string")

	// ErrInvalidTimeFormat is returned when the CSV price record does not have a valid time unix milli format.
	ErrInvalidTimeFormat = errors.New("cannot parse time string")

	// ErrInvalidOrderSideFormat is returned when the CSV side record does not have a valid buy or sell string.
	ErrInvalidOrderSideFormat = errors.New("cannot parse order side string")

	// ErrInvalidPriceFormat is returned when the CSV price record does not prices in expected format.
	ErrInvalidPriceFormat = errors.New("OHLC prices must be valid number format")

	// ErrInvalidVolumeFormat is returned when the CSV price record does not have a valid volume format.
	ErrInvalidVolumeFormat = errors.New("volume must be valid number format")
)

// CSVKLineDecoder is an extension point for CSVKLineReader to support custom file formats.
type CSVKLineDecoder func(record []string, interval time.Duration) (types.KLine, error)

// NewBinanceCSVKLineReader creates a new CSVKLineReader for Binance CSV files.
func NewBinanceCSVKLineReader(csv *csv.Reader) *CSVKLineReader {
	return &CSVKLineReader{
		csv:     csv,
		decoder: BinanceCSVKLineDecoder,
	}
}

// BinanceCSVKLineDecoder decodes a CSV record from Binance or Bybit into a KLine.
func BinanceCSVKLineDecoder(record []string, interval time.Duration) (types.KLine, error) {
	var (
		k, empty types.KLine
		err      error
	)
	if len(record) < 5 {
		return k, ErrNotEnoughColumns
	}
	ts, err := strconv.ParseFloat(record[0], 64) // check for e numbers "1.70027E+12"
	if err != nil {
		return empty, ErrInvalidTimeFormat
	}
	open, err := fixedpoint.NewFromString(record[1])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	high, err := fixedpoint.NewFromString(record[2])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	low, err := fixedpoint.NewFromString(record[3])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	closing, err := fixedpoint.NewFromString(record[4])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}

	volume, err := fixedpoint.NewFromString(record[5])
	if err != nil {
		return empty, ErrInvalidVolumeFormat
	}

	k.StartTime = types.NewTimeFromUnix(int64(ts), 0)
	k.EndTime = types.NewTimeFromUnix(k.StartTime.Time().Add(interval).Unix(), 0)
	k.Open = open
	k.High = high
	k.Low = low
	k.Close = closing
	k.Volume = volume

	return k, nil
}

// NewMetaTraderCSVKLineReader creates a new CSVKLineReader for MetaTrader CSV files.
func NewMetaTraderCSVKLineReader(csv *csv.Reader) *CSVKLineReader {
	csv.Comma = ';'
	return &CSVKLineReader{
		csv:     csv,
		decoder: MetaTraderCSVKLineDecoder,
	}
}

// MetaTraderCSVKLineDecoder decodes a CSV record from MetaTrader into a KLine.
func MetaTraderCSVKLineDecoder(record []string, interval time.Duration) (types.KLine, error) {
	var (
		k, empty types.KLine
		err      error
	)

	if len(record) < 6 {
		return k, ErrNotEnoughColumns
	}

	tStr := fmt.Sprintf("%s %s", record[0], record[1])
	t, err := time.Parse(MetaTraderTimeFormat, tStr)
	if err != nil {
		return empty, ErrInvalidTimeFormat
	}
	open, err := fixedpoint.NewFromString(record[2])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	high, err := fixedpoint.NewFromString(record[3])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	low, err := fixedpoint.NewFromString(record[4])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	closing, err := fixedpoint.NewFromString(record[5])
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	volume, err := fixedpoint.NewFromString(record[6])
	if err != nil {
		return empty, ErrInvalidVolumeFormat
	}

	k.StartTime = types.NewTimeFromUnix(t.Unix(), 0)
	k.EndTime = types.NewTimeFromUnix(t.Add(interval).Unix(), 0)
	k.Open = open
	k.High = high
	k.Low = low
	k.Close = closing
	k.Volume = volume

	return k, nil
}
