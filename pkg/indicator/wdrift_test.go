package indicator

import (
	"encoding/json"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_WDrift(t *testing.T) {
	var randomPrices = []byte(`[1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 4, 1, 2, 3, 4, 5, 6, 7, 8, 9, 4, 1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 4, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
	var input []fixedpoint.Value
	if err := json.Unmarshal(randomPrices, &input); err != nil {
		panic(err)
	}
	buildKLines := func(prices []fixedpoint.Value) (klines []types.KLine) {
		for _, p := range prices {
			klines = append(klines, types.KLine{Close: p, Volume: fixedpoint.One})
		}

		return klines
	}
	tests := []struct {
		name   string
		kLines []types.KLine
		all    int
	}{
		{
			name:   "random_case",
			kLines: buildKLines(input),
			all:    47,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drift := WeightedDrift{IntervalWindow: types.IntervalWindow{Window: 3}}
			drift.CalculateAndUpdate(tt.kLines)
			assert.Equal(t, drift.Length(), tt.all)
			for _, v := range drift.Values {
				assert.LessOrEqual(t, v, 1.0)
			}
		})
	}
}
