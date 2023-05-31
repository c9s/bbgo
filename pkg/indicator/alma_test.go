package indicator

import (
	"encoding/json"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

/*
python:

import pandas as pd
import pandas_ta as ta

data = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
sigma = 6
offset = 0.9
size = 5

result = ta.alma(data, size, sigma, offset)
print(result)
*/
func Test_ALMA(t *testing.T) {
	var Delta = 0.01
	var randomPrices = []byte(`[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
	var input []fixedpoint.Value
	if err := json.Unmarshal(randomPrices, &input); err != nil {
		panic(err)
	}
	tests := []struct {
		name   string
		kLines []types.KLine
		want   float64
		next   float64
		all    int
	}{
		{
			name:   "random_case",
			kLines: buildKLines(input),
			want:   5.60785,
			next:   4.60785,
			all:    26,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alma := ALMA{
				IntervalWindow: types.IntervalWindow{Window: 5},
				Offset:         0.9,
				Sigma:          6,
			}
			alma.CalculateAndUpdate(tt.kLines)
			assert.InDelta(t, tt.want, alma.Last(0), Delta)
			assert.InDelta(t, tt.next, alma.Index(1), Delta)
			assert.Equal(t, tt.all, alma.Length())
		})
	}
}
