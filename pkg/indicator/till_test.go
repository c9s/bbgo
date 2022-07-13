package indicator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

/*
python:

import pandas as pd
s = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
ma1 = s.ewm(span=16).mean()
ma2 = ma1.ewm(span=16).mean()
ma3 = ma2.ewm(span=16).mean()
ma4 = ma3.ewm(span=16).mean()
ma5 = ma4.ewm(span=16).mean()
ma6 = ma5.ewm(span=16).mean()
square = 0.7 * 0.7
cube = 0.7 ** 3
c1 = -cube
c2 = 3 * square + 3 * cube
c3 = -6 * square - 3 * 0.7 - 3 * cube
c4 = 1 + 3 * 0.7 + cube + 3 * square
result = (c1 * ma6 + c2 * ma5 + c3 * ma4 + c4 * ma3)
print(result)
*/
func Test_TILL(t *testing.T) {
	var Delta = 0.18
	var randomPrices = []byte(`[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
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
			want:   4.528608,
			next:   4.457134,
			all:    50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			till := TILL{IntervalWindow: types.IntervalWindow{Window: 16}}
			till.CalculateAndUpdate(tt.kLines)
			last := till.Last()
			assert.InDelta(t, tt.want, last, Delta)
			assert.InDelta(t, tt.next, till.Index(1), Delta)
			assert.Equal(t, tt.all, till.Length())
		})
	}
}
