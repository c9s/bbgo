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
	ErrInvalidTimeFormat = errors.New("cannot parse time string")

	// ErrInvalidPriceFormat is returned when the CSV price record does not prices in expected format.
	ErrInvalidPriceFormat = errors.New("OHLC prices must be in valid decimal format")

	// ErrInvalidVolumeFormat is returned when the CSV price record does not have a valid volume format.
	ErrInvalidVolumeFormat = errors.New("volume must be in valid float format")
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

	msec, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return empty, ErrInvalidTimeFormat
	}
	k.StartTime = types.NewTimeFromUnix(time.UnixMilli(msec).Unix(), 0)
	k.EndTime = types.NewTimeFromUnix(k.StartTime.Time().Add(interval).Unix(), 0)

	open, err := strconv.ParseFloat(record[1], 64)
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	k.Open = fixedpoint.NewFromFloat(open)

	high, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	k.High = fixedpoint.NewFromFloat(high)

	low, err := strconv.ParseFloat(record[3], 64)
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	k.Low = fixedpoint.NewFromFloat(low)

	close, err := strconv.ParseFloat(record[4], 64)
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	k.Close = fixedpoint.NewFromFloat(close)

	if len(record) > 5 {
		vol, err := strconv.ParseFloat(record[5], 64)
		if err != nil {
			return empty, ErrInvalidVolumeFormat
		}
		k.Volume = fixedpoint.NewFromFloat(vol)
	}

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
	k.StartTime = types.NewTimeFromUnix(t.Unix(), 0)
	k.EndTime = types.NewTimeFromUnix(t.Add(interval).Unix(), 0)

	open, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	k.Open = fixedpoint.NewFromFloat(open)

	high, err := strconv.ParseFloat(record[3], 64)
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	k.High = fixedpoint.NewFromFloat(high)

	low, err := strconv.ParseFloat(record[4], 64)
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	k.Low = fixedpoint.NewFromFloat(low)

	close, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return empty, ErrInvalidPriceFormat
	}
	k.Close = fixedpoint.NewFromFloat(close)

	if len(record) > 5 {
		vol, err := strconv.ParseFloat(record[6], 64)
		if err != nil {
			return empty, ErrInvalidVolumeFormat
		}
		k.Volume = fixedpoint.NewFromFloat(vol)
	}

	return k, nil
}
