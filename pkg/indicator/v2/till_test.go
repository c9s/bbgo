package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/indicator"
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
	source := types.NewFloat64Series()
	till := TILL(source, 16, 0)

	for i := 0; i < 50; i++ {
		source.PushAndEmit(float64(i % 10))
	}

	const delta = 0.18
	assert.Equal(t, 50, till.Length())
	assert.InDelta(t, 4.528608, till.Last(0), delta)
	assert.InDelta(t, 4.457134, till.Last(1), delta)
}

func Test_TILL_Legacy(t *testing.T) {
	source := types.NewFloat64Series()
	till := TILL(source, 16, 0.5)
	legacy := &indicator.TILL{IntervalWindow: types.IntervalWindow{Window: 16}, VolumeFactor: 0.5}

	for i := 0; i < 200; i++ {
		v := 100 + float64(i%10) + float64(i%7)*0.3
		source.PushAndEmit(v)
		legacy.Update(v)
		assert.InDelta(t, legacy.Last(0), till.Last(0), 1e-9)
	}
}
