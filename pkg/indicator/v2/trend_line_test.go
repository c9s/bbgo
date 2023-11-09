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
}
