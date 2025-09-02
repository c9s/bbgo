package csvsource

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestReadTicksFromBinanceCSV(t *testing.T) {
	path := "./testdata/binance/FXSUSDT-ticks-2023-10-29.csv"
	symbol := "FXSUSDT"
	intervals := []types.Interval{types.Interval1h}
	klineMap, err := ReadTicksFromCSVWithDecoder(
		path, symbol, intervals, MakeCSVTickReader(NewBinanceCSVTickReader),
	)
	klines := klineMap[types.Interval1h]
	assert.NoError(t, err)
	assert.Len(t, klines, 1)
	assert.Equal(t, int64(1698620400), klines[0].StartTime.Unix(), "StartTime")
	assert.Equal(t, int64(1698624000), klines[0].EndTime.Unix(), "EndTime")
	assert.Equal(t, 6.0, klines[0].Open.Float64(), "Open")
	assert.Equal(t, 6.0, klines[0].High.Float64(), "High")
	assert.Equal(t, 6.0, klines[0].Low.Float64(), "Low")
	assert.Equal(t, 6.0, klines[0].Close.Float64(), "Close")
	assert.Equal(t, 111.0, klines[0].Volume.Float64(), "Volume")
}

func TestReadTicksFromBybitCSV(t *testing.T) {
	path := "./testdata/bybit/FXSUSDT2023-10-10.csv"
	symbol := "FXSUSDT"
	intervals := []types.Interval{types.Interval1h}
	klineMap, err := ReadTicksFromCSVWithDecoder(
		path, symbol, intervals, MakeCSVTickReader(NewBybitCSVTickReader),
	)
	klines := klineMap[types.Interval1h]
	assert.NoError(t, err)
	assert.Len(t, klines, 1)
	assert.Equal(t, int64(1696978800), klines[0].StartTime.Unix(), "StartTime")
	assert.Equal(t, int64(1696982400), klines[0].EndTime.Unix(), "EndTime")
	assert.Equal(t, 5.239, klines[0].Open.Float64(), "Open")
	assert.Equal(t, 5.2495, klines[0].High.Float64(), "High")
	assert.Equal(t, 5.239, klines[0].Low.Float64(), "Low")
	assert.Equal(t, 5.2495, klines[0].Close.Float64(), "Close")
	assert.Equal(t, 147.05, klines[0].Volume.Float64(), "Volume")
}

func TestReadTicksFromOkexCSV(t *testing.T) {
	path := "./testdata/okex/BTC-USDT-aggtrades-2023-11-18.csv"
	symbol := "BTCUSDT"
	intervals := []types.Interval{types.Interval1h}
	klineMap, err := ReadTicksFromCSVWithDecoder(
		path, symbol, intervals, MakeCSVTickReader(NewOKExCSVTickReader),
	)
	klines := klineMap[types.Interval1h]
	assert.NoError(t, err)
	assert.Len(t, klines, 1)
	assert.Equal(t, int64(1700236800), klines[0].StartTime.Unix(), "StartTime")
	assert.Equal(t, int64(1700240400), klines[0].EndTime.Unix(), "EndTime")
	assert.Equal(t, 35910.6, klines[0].Open.Float64(), "Open")
	assert.Equal(t, 35914.4, klines[0].High.Float64(), "High")
	assert.Equal(t, 35910.6, klines[0].Low.Float64(), "Low")
	assert.Equal(t, 35914.4, klines[0].Close.Float64(), "Close")
	assert.Equal(t, 51525.38700081, klines[0].Volume.Float64(), "Volume")

}
