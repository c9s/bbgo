package csvsource

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestReadTicksFromCSV(t *testing.T) {
	expectedStartTime := time.Unix(1698623881260, 0)
	expectedEndTime := expectedStartTime.Add(time.Minute)
	// 11771900,6.06300000,7.70000000,14959258,14959262,1698537604628,False,True

	klines, err := ReadTicksFromCSV("./testdata/FXSUSDT-ticks-2023-10-29.csv", types.Interval1m)
	assert.NoError(t, err)
	assert.Len(t, klines, 1)
	assert.Equal(t, expectedStartTime.Unix(), klines[0].StartTime.Unix(), "StartTime")
	assert.Equal(t, expectedEndTime.Unix(), klines[0].EndTime.Unix(), "EndTime")
	assert.Equal(t, 6.0, klines[0].Open.Float64(), "Open")
	assert.Equal(t, 6.0, klines[0].High.Float64(), "High")
	assert.Equal(t, 6.0, klines[0].Low.Float64(), "Low")
	assert.Equal(t, 6.0, klines[0].Close.Float64(), "Close")
	assert.Equal(t, 111.0, klines[0].Volume.Float64(), "Volume")
}
