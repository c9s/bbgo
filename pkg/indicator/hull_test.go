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
ma1 = s.ewm(span=8).mean()
ma2 = s.ewm(span=16).mean()
result = (2 * ma1 - ma2).ewm(span=4).mean()
print(result)
*/
func Test_HULL(t *testing.T) {
	var Delta = 1.5e-2
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
			want:   6.002935,
			next:   5.167056,
			all:    50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hull := &HULL{IntervalWindow: types.IntervalWindow{Window: 16}}
			for _, k := range tt.kLines {
				hull.PushK(k)
			}

			last := hull.Last(0)
			assert.InDelta(t, tt.want, last, Delta)
			assert.InDelta(t, tt.next, hull.Index(1), Delta)
			assert.Equal(t, tt.all, hull.Length())
		})
	}
}
