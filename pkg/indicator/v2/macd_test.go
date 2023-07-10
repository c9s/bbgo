package indicatorv2

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

/*
python:

import pandas as pd
s = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
slow = s.ewm(span=26, adjust=False).mean()
fast = s.ewm(span=12, adjust=False).mean()
print(fast - slow)
*/

func buildKLines(prices []fixedpoint.Value) (klines []types.KLine) {
	for _, p := range prices {
		klines = append(klines, types.KLine{Close: p})
	}

	return klines
}

func Test_MACD2(t *testing.T) {
	var randomPrices = []byte(`[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
	var input []fixedpoint.Value
	err := json.Unmarshal(randomPrices, &input)
	assert.NoError(t, err)

	tests := []struct {
		name   string
		kLines []types.KLine
		want   float64
	}{
		{
			name:   "random_case",
			kLines: buildKLines(input),
			want:   0.7740187187598249,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prices := ClosePrices(nil)
			macd := MACD2(prices, 12, 26, 9)
			for _, k := range tt.kLines {
				prices.EmitUpdate(k.Close.Float64())
			}

			got := macd.Last(0)
			diff := math.Trunc((got-tt.want)*100) / 100
			if diff != 0 {
				t.Errorf("MACD2() = %v, want %v", got, tt.want)
			}
		})
	}
}
