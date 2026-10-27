package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
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
	const delta = 0.00001
	input := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	tests := []struct {
		name  string
		poles int
		want  float64
		next  float64
		all   int
	}{
		{name: "pole2", poles: 2, want: 8.721776, next: 7.723223, all: 30},
		{name: "pole3", poles: 3, want: 8.687588, next: 7.668013, all: 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := types.NewFloat64Series()
			ssf := SSF(source, 5, tt.poles)
			for _, v := range input {
				source.PushAndEmit(v)
			}

			assert.InDelta(t, tt.want, ssf.Last(0), delta)
			assert.InDelta(t, tt.next, ssf.Index(1), delta)
			assert.Equal(t, tt.all, ssf.Length())
		})
	}
}
