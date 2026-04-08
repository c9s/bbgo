package csvsource

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_ReadTicksFromCSV(t *testing.T) {
	// test on binance data is enough since the CSV format is normalized by the downloader
	path := "./testdata/binance/FXSUSDT/aggTrades"
	symbol := "FXSUSDT"
	intervals := []types.Interval{types.Interval1h}
	klineMap, err := ReadTicksFromCSV(
		path, symbol, intervals,
	)
	klines := klineMap[types.Interval1h]
	assert.NoError(t, err)
	assert.Len(t, klines, 4)
	assert.Equal(t, int64(1700179200), klines[0].StartTime.Unix(), "StartTime")
	assert.Equal(t, int64(1700182800), klines[0].EndTime.Unix(), "EndTime")
	assert.Equal(t, 7.503, klines[0].Open.Float64(), "Open")
	assert.Equal(t, 7.601, klines[0].High.Float64(), "High")
	assert.Equal(t, 7.495, klines[0].Low.Float64(), "Low")
	assert.Equal(t, 7.598, klines[0].Close.Float64(), "Close")
	assert.Equal(t, 26786.3, klines[0].Volume.Float64(), "Volume")
}
