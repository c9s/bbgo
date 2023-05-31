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
lag = int((16-1)/2 + 0.5)
emadata = s + (s - s.shift(lag))
result = emadata.ewm(span=16).mean()
print(result)
*/
func Test_ZLEMA(t *testing.T) {
	var Delta = 6.5e-2
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
			want:   6.622881,
			next:   5.231044,
			all:    42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zlema := ZLEMA{IntervalWindow: types.IntervalWindow{Window: 16}}
			zlema.CalculateAndUpdate(tt.kLines)
			last := zlema.Last(0)
			assert.InDelta(t, tt.want, last, Delta)
			assert.InDelta(t, tt.next, zlema.Index(1), Delta)
			assert.Equal(t, tt.all, zlema.Length())
		})
	}
}
