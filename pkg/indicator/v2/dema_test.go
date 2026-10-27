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
result = (2 * ma1 - ma2)
print(result)
*/
func Test_DEMA(t *testing.T) {
	source := types.NewFloat64Series()
	dema := DEMA(source, 16)

	for i := 0; i < 50; i++ {
		source.PushAndEmit(float64(i % 10))
	}

	const delta = 4e-2
	assert.Equal(t, 50, dema.Length())
	assert.InDelta(t, 6.420838, dema.Last(0), delta)
	assert.InDelta(t, 5.609367, dema.Last(1), delta)
	assert.Equal(t, 50, dema.A1.Length())
	assert.Equal(t, 50, dema.A2.Length())
}

func Test_DEMA_Replay(t *testing.T) {
	// a DEMA bound to a source that already has history should produce the same values
	source := types.NewFloat64Series()
	live := DEMA(source, 16)
	for i := 0; i < 50; i++ {
		source.PushAndEmit(float64(i%10) + 1)
	}

	replayed := DEMA(source, 16)
	assert.Equal(t, live.Slice, replayed.Slice)
}

func Test_DEMA_Legacy(t *testing.T) {
	source := types.NewFloat64Series()
	dema := DEMA(source, 16)
	legacy := &indicator.DEMA{IntervalWindow: types.IntervalWindow{Window: 16}}

	for i := 0; i < 200; i++ {
		v := 100 + float64(i%10) + float64(i%7)*0.3
		source.PushAndEmit(v)
		legacy.Update(v)
		assert.InDelta(t, legacy.Last(0), dema.Last(0), 1e-9)
	}
}
