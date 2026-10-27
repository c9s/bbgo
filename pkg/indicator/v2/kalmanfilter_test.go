package indicatorv2

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_KalmanFilter(t *testing.T) {
	klines := loadETHUSDT5mKLines(t)

	tests := []struct {
		name                   string
		window                 int
		additionalSmoothWindow uint
		want                   float64
	}{
		{name: "ETHUSDT Kalman Filter 7", window: 7, additionalSmoothWindow: 3, want: 1369.24},
		{name: "ETHUSDT Kalman Filter 25", window: 25, additionalSmoothWindow: 0, want: 1369.84},
		{name: "ETHUSDT Kalman Filter 99", window: 99, additionalSmoothWindow: 0, want: 1369.95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := &KLineStream{}
			filter := KalmanFilter(stream, tt.window, tt.additionalSmoothWindow)
			stream.BackFill(klines)

			assert.Equal(t, len(klines), filter.Length())
			got := math.Trunc(filter.Last(0)*100.0) / 100.0
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_KalmanFilterEstimationAccurate(t *testing.T) {
	klines := loadETHUSDT5mKLines(t)

	tests := []struct {
		name        string
		partialInfo bool
		window      int
	}{
		{name: "full K-line 7", window: 7},
		{name: "full K-line 25", window: 25},
		{name: "full K-line 99", window: 99},
		{name: "partial K-line 7", partialInfo: true, window: 7},
		{name: "partial K-line 25", partialInfo: true, window: 25},
		{name: "partial K-line 99", partialInfo: true, window: 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := &KLineStream{}
			var filter *KalmanFilterStream
			if tt.partialInfo {
				filter = KalmanFilterFromPrices(ClosePrices(stream), tt.window, 0)
			} else {
				filter = KalmanFilter(stream, tt.window, 0)
			}
			ewma := EWMA2(ClosePrices(stream), tt.window)

			var filterDiff2Sum, ewmaDiff2Sum float64
			var filterCloseDiff2Sum, ewmaCloseDiff2Sum float64
			var numEstimations = 0
			for _, k := range klines {
				// square error between last estimated state and current actual state
				if ewma.Length() > 0 {
					filterDiff2Sum += klineSquareError(filter.Last(0), k)
					ewmaDiff2Sum += klineSquareError(ewma.Last(0), k)
					filterCloseDiff2Sum += closeSquareError(filter.Last(0), k)
					ewmaCloseDiff2Sum += closeSquareError(ewma.Last(0), k)
					numEstimations++
				}

				// update estimations
				stream.BackFill([]types.KLine{k})
			}

			n := float64(numEstimations)
			assert.LessOrEqual(t, math.Sqrt(filterDiff2Sum/(n*4)), math.Sqrt(ewmaDiff2Sum/(n*4)))
			assert.LessOrEqual(t, math.Sqrt(filterCloseDiff2Sum/n), math.Sqrt(ewmaCloseDiff2Sum/n))
		})
	}
}
