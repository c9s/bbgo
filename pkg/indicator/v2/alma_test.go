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
sigma = 6
offset = 0.9
size = 5

result = ta.alma(data, size, sigma, offset)
print(result)
*/
func Test_ALMA(t *testing.T) {
	const delta = 0.01
	input := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	source := types.NewFloat64Series()
	alma := ALMA(source, 5, 0.9, 6)

	var emitted []float64
	alma.OnUpdate(func(v float64) { emitted = append(emitted, v) })

	for _, v := range input {
		source.PushAndEmit(v)
	}

	assert.InDelta(t, 5.60785, alma.Last(0), delta)
	assert.InDelta(t, 4.60785, alma.Index(1), delta)
	assert.Equal(t, 26, alma.Length())
	assert.Len(t, emitted, 26)
}
