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
ma3 = ma2.ewm(span=16).mean()
result = (3 * ma1 - 3 * ma2 + ma3)
print(result)
*/
func Test_TEMA(t *testing.T) {
	var Delta = 4.3e-2
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
			want:   7.163145,
			next:   6.106229,
			all:    50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tema := TEMA{IntervalWindow: types.IntervalWindow{Window: 16}}
			tema.CalculateAndUpdate(tt.kLines)
			last := tema.Last(0)
			assert.InDelta(t, tt.want, last, Delta)
			assert.InDelta(t, tt.next, tema.Index(1), Delta)
			assert.Equal(t, tt.all, tema.Length())
		})
	}
}
