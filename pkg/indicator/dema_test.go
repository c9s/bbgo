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
s = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9])
ma1 = s.ewm(span=16).mean()
ma2 = ma1.ewm(span=16).mean()
result = (2 * ma1 - ma2)
print(result)
*/
func Test_DEMA(t *testing.T) {
	var Delta = 4e-2
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
			want:   6.420838,
			next:   5.609367,
			all:    50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dema := DEMA{IntervalWindow: types.IntervalWindow{Window: 16}}
			dema.CalculateAndUpdate(tt.kLines)
			last := dema.Last()
			assert.InDelta(t, tt.want, last, Delta)
			assert.InDelta(t, tt.next, dema.Index(1), Delta)
			assert.Equal(t, tt.all, dema.Length())
		})
	}
}
