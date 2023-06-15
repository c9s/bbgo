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
size = 5

result = ta.ssf(data, size, 2)
print(result)

result = ta.ssf(data, size, 3)
print(result)
*/
func Test_SSF(t *testing.T) {
	var Delta = 0.00001
	var randomPrices = []byte(`[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
	var input []fixedpoint.Value
	if err := json.Unmarshal(randomPrices, &input); err != nil {
		panic(err)
	}
	tests := []struct {
		name   string
		kLines []types.KLine
		poles  int
		want   float64
		next   float64
		all    int
	}{
		{
			name:   "pole2",
			kLines: buildKLines(input),
			poles:  2,
			want:   8.721776,
			next:   7.723223,
			all:    30,
		},
		{
			name:   "pole3",
			kLines: buildKLines(input),
			poles:  3,
			want:   8.687588,
			next:   7.668013,
			all:    30,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ssf := SSF{
				IntervalWindow: types.IntervalWindow{Window: 5},
				Poles:          tt.poles,
			}
			ssf.CalculateAndUpdate(tt.kLines)
			assert.InDelta(t, tt.want, ssf.Last(0), Delta)
			assert.InDelta(t, tt.next, ssf.Index(1), Delta)
			assert.Equal(t, tt.all, ssf.Length())
		})
	}
}
