package indicatorv2

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/types"
)

// loadETHUSDT5mKLines loads 1000 ETHUSDT 5m klines from Binance, ending at 2022/07/27 00:00
// https://www.binance.com/api/v3/klines?symbol=ETHUSDT&interval=5m&endTime=1658851200000&limit=1000
func loadETHUSDT5mKLines(t *testing.T) []types.KLine {
	data, err := os.ReadFile("testdata/ETHUSDT-5m-2022-07-27.json")
	require.NoError(t, err)

	var klines []types.KLine
	require.NoError(t, json.Unmarshal(data, &klines))
	require.Len(t, klines, 1000)
	return klines
}

func klineSquareError(base float64, k types.KLine) float64 {
	openDiff := math.Abs(k.Open.Float64() - base)
	highDiff := math.Abs(k.High.Float64() - base)
	lowDiff := math.Abs(k.Low.Float64() - base)
	closeDiff := math.Abs(k.Close.Float64() - base)
	return openDiff*openDiff + highDiff*highDiff + lowDiff*lowDiff + closeDiff*closeDiff
}

func closeSquareError(base float64, k types.KLine) float64 {
	closeDiff := math.Abs(k.Close.Float64() - base)
	return closeDiff * closeDiff
}

func Test_GHFilter(t *testing.T) {
	klines := loadETHUSDT5mKLines(t)

	tests := []struct {
		name   string
		window int
		want   float64
	}{
		{name: "ETHUSDT G-H Filter 7", window: 7, want: 1373.71},
		{name: "ETHUSDT G-H Filter 25", window: 25, want: 1376.21},
		{name: "ETHUSDT G-H Filter 99", window: 99, want: 1378.96},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := &KLineStream{}
			filter := GHFilter(stream, tt.window)
			stream.BackFill(klines)

			assert.Equal(t, len(klines), filter.Length())
			got := math.Trunc(filter.Last(0)*100.0) / 100.0
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_GHFilterEstimationAccurate(t *testing.T) {
	klines := loadETHUSDT5mKLines(t)

	for _, window := range []int{7, 25, 99} {
		stream := &KLineStream{}
		filter := GHFilter(stream, window)
		ewma := EWMA2(ClosePrices(stream), window)

		var filterDiff2Sum, ewmaDiff2Sum float64
		var filterCloseDiff2Sum, ewmaCloseDiff2Sum float64
		for i, k := range klines {
			// square error between last estimated state and current actual state
			if i > 0 {
				filterDiff2Sum += klineSquareError(filter.Last(0), k)
				ewmaDiff2Sum += klineSquareError(ewma.Last(0), k)
				filterCloseDiff2Sum += closeSquareError(filter.Last(0), k)
				ewmaCloseDiff2Sum += closeSquareError(ewma.Last(0), k)
			}

			// update estimations
			stream.BackFill([]types.KLine{k})
		}

		numEstimations := float64(len(klines) - 1)
		assert.LessOrEqual(t, math.Sqrt(filterDiff2Sum/(numEstimations*4)), math.Sqrt(ewmaDiff2Sum/(numEstimations*4)), "window %d", window)
		assert.LessOrEqual(t, math.Sqrt(filterCloseDiff2Sum/numEstimations), math.Sqrt(ewmaCloseDiff2Sum/numEstimations), "window %d", window)
	}
}

func Test_GHFilterFromPrices(t *testing.T) {
	// hand-computed with window = 3 (multiplier = 0.5), uncertainty = |v - last measurement|
	// v0 = 10: a = 0, b = 5, out = 10
	// v1 = 12: a = 0.5*2 = 1, b = 0.5*1 + 0.5*5 = 3, lambda = 1/3
	values := []float64{10, 12}
	source := types.NewFloat64Series()
	filter := GHFilterFromPrices(source, 3)
	for _, v := range values {
		source.PushAndEmit(v)
	}

	lambda2 := 1.0 / 9.0
	alpha := (-lambda2 + math.Sqrt(lambda2*lambda2+16*lambda2)) / 8
	assert.Equal(t, 2, filter.Length())
	assert.InDelta(t, 10.0, filter.Last(1), 1e-9)
	assert.InDelta(t, alpha*12+(1-alpha)*10, filter.Last(0), 1e-9)
}
