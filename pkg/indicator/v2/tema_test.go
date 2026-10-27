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
result = (3 * ma1 - 3 * ma2 + ma3)
print(result)
*/
func Test_TEMA(t *testing.T) {
	source := types.NewFloat64Series()
	tema := TEMA(source, 16)

	for i := 0; i < 50; i++ {
		source.PushAndEmit(float64(i % 10))
	}

	const delta = 4.3e-2
	assert.Equal(t, 50, tema.Length())
	assert.InDelta(t, 7.163145, tema.Last(0), delta)
	assert.InDelta(t, 6.106229, tema.Last(1), delta)
}

func Test_TEMA_Legacy(t *testing.T) {
	source := types.NewFloat64Series()
	tema := TEMA(source, 16)
	legacy := &indicator.TEMA{IntervalWindow: types.IntervalWindow{Window: 16}}

	for i := 0; i < 200; i++ {
		v := 100 + float64(i%10) + float64(i%7)*0.3
		source.PushAndEmit(v)
		legacy.Update(v)
		assert.InDelta(t, legacy.Last(0), tema.Last(0), 1e-9)
	}
}
