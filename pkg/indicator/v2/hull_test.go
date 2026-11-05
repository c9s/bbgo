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
ma1 = s.ewm(span=8).mean()
ma2 = s.ewm(span=16).mean()
result = (2 * ma1 - ma2).ewm(span=4).mean()
print(result)
*/
func Test_HULL(t *testing.T) {
	source := types.NewFloat64Series()
	hull := HULL(source, 16)

	for i := 0; i < 50; i++ {
		source.PushAndEmit(float64(i % 10))
	}

	const delta = 1.5e-2
	assert.Equal(t, 50, hull.Length())
	assert.InDelta(t, 6.002935, hull.Last(0), delta)
	assert.InDelta(t, 5.167056, hull.Last(1), delta)
}

func Test_HULL_Legacy(t *testing.T) {
	source := types.NewFloat64Series()
	hull := HULL(source, 16)
	legacy := &indicator.HULL{IntervalWindow: types.IntervalWindow{Window: 16}}

	for i := 0; i < 200; i++ {
		v := 100 + float64(i%10) + float64(i%7)*0.3
		source.PushAndEmit(v)
		legacy.Update(v)
		assert.InDelta(t, legacy.Last(0), hull.Last(0), 1e-9)
	}
}
