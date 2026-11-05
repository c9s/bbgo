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
lag = int((16-1)/2 + 0.5)
emadata = s + (s - s.shift(lag))
result = emadata.ewm(span=16).mean()
print(result)
*/
func Test_ZLEMA(t *testing.T) {
	source := types.NewFloat64Series()
	zlema := ZLEMA(source, 16)

	for i := 0; i < 50; i++ {
		source.PushAndEmit(float64(i % 10))
	}

	const delta = 6.5e-2
	// lag = 8, so the first 8 inputs produce no output
	assert.Equal(t, 42, zlema.Length())
	assert.InDelta(t, 6.622881, zlema.Last(0), delta)
	assert.InDelta(t, 5.231044, zlema.Last(1), delta)
}

func Test_ZLEMA_Legacy(t *testing.T) {
	source := types.NewFloat64Series()
	zlema := ZLEMA(source, 16)
	legacy := &indicator.ZLEMA{IntervalWindow: types.IntervalWindow{Window: 16}}

	for i := 0; i < 200; i++ {
		v := 100 + float64(i%10) + float64(i%7)*0.3
		source.PushAndEmit(v)
		legacy.Update(v)
		assert.Equal(t, legacy.Length(), zlema.Length())
		assert.InDelta(t, legacy.Last(0), zlema.Last(0), 1e-9)
	}
}
