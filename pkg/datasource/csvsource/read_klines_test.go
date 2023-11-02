package csvsource

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReadKLinesFromCSV(t *testing.T) {
	expectedEndTime := time.Unix(1609459200, 0).Add(time.Hour)

	klines, err := ReadKLinesFromCSV("./testdata/BTCUSDT-1h-2021-Q1.csv", time.Hour)
	assert.NoError(t, err)
	assert.Len(t, klines, 2158)
	assert.Equal(t, int64(1609459200), klines[0].StartTime.Unix(), "StartTime")
	assert.Equal(t, expectedEndTime.Unix(), klines[0].EndTime.Unix(), "EndTime")
	assert.Equal(t, 28923.63, klines[0].Open.Float64(), "Open")
	assert.Equal(t, 29031.34, klines[0].High.Float64(), "High")
	assert.Equal(t, 28690.17, klines[0].Low.Float64(), "Low")
	assert.Equal(t, 28995.13, klines[0].Close.Float64(), "Close")
	assert.Equal(t, 2311.81144499, klines[0].Volume.Float64(), "Volume")
}
