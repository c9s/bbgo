package indicatorv2

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestTrendIndicator(t *testing.T) {
	t.Run("returns the correct slope of the trend", func(t *testing.T) {
		tests := []struct {
			closing        []float64
			expectedResult float64
		}{
			{
				closing:        []float64{0, 1, 2, 3},
				expectedResult: 1,
			},
			{
				closing:        []float64{0, 2, 4, 6},
				expectedResult: 2,
			},
			{
				closing:        []float64{5, 4, 3, 2},
				expectedResult: -1,
			},
		}

		for _, tt := range tests {
			stream := &types.StandardStream{}
			kLines := KLines(stream, "", "")
			ind := TrendLine(kLines, 3)
			buildKLines := func(closing []float64) (kLines []types.KLine) {
				for i := range closing {
					kLines = append(kLines, types.KLine{Close: n(closing[i])})
				}
				return kLines
			}
			ts := buildKLines(tt.closing)
			for _, d := range ts {
				stream.EmitKLineClosed(d)
			}

			spew.Dump(ind)
			assert.InDelta(t, tt.expectedResult, ind.Last(0), 0.001, "Expected Trend.Last(0) to be %v, but got %v", tt.expectedResult, ind.Last(0))

		}
	})

	// t.Run("respects the window", func(t *testing.T) {
	// 	indicator := NewTrendLine(4)
	// 	indicator.Update(dec.Slice(-100, 1000, 0, 1, 2, 3)...)
	// 	source := types.NewFloat64Series()
	// 	sma := SMA(source, 9)

	// 	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	// 	for _, d := range data {
	// 		source.PushAndEmit(d)
	// 	}

	// 	assert.InDelta(t, 5, sma.Last(0), 0.001)
	// 	assert.EqualValues(t, 1, indicator.Value().Float64())
	// })

	// t.Run("does not allow an index out of bounds on the low end", func(t *testing.T) {
	// 	indicator := NewTrendLine(2)
	// 	indicator.Update(dec.Slice(0, 0, 1)...)
	// 	source := types.NewFloat64Series()
	// 	sma := SMA(source, 9)

	// 	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	// 	for _, d := range data {
	// 		source.PushAndEmit(d)
	// 	}

	// 	assert.InDelta(t, 5, sma.Last(0), 0.001)
	// 	assert.EqualValues(t, 1, indicator.Value().Float64())
	// })
}
