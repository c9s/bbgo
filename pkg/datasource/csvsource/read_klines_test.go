package csvsource

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReadKLinesFromCSV(t *testing.T) {
	klines, err := ReadKLinesFromCSV("./testdata/binance/BTCUSDT-1h-2023-11-18.csv", time.Hour)
	assert.NoError(t, err)
	assert.Len(t, klines, 24)
	assert.Equal(t, int64(1700265600), klines[0].StartTime.Unix(), "StartTime")
	assert.Equal(t, int64(1700269200), klines[0].EndTime.Unix(), "EndTime")
	assert.Equal(t, 36613.91, klines[0].Open.Float64(), "Open")
	assert.Equal(t, 36613.92, klines[0].High.Float64(), "High")
	assert.Equal(t, 36388.12, klines[0].Low.Float64(), "Low")
	assert.Equal(t, 36400.01, klines[0].Close.Float64(), "Close")
	assert.Equal(t, 1005.75727, klines[0].Volume.Float64(), "Volume")
}
