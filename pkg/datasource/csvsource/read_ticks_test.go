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
	klineMap, err := ReadTicksFromCSV(
		path, symbol, intervals,
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
