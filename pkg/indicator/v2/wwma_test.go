package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func TestWWMA(t *testing.T) {
	source := types.NewFloat64Series()
	wwma := WWMA(source, 4)
	for _, v := range []float64{10, 14, 6} {
		source.PushAndEmit(v)
	}

	// 10, 10 + (14 - 10) / 4 = 11, 11 + (6 - 11) / 4 = 9.75
	assert.InDeltaSlice(t, []float64{10, 11, 9.75}, []float64(wwma.Slice), 1e-9)
}

func TestWWMA_CompareLegacy(t *testing.T) {
	prices := randomWalkPrices(200, 7)
	window := 14

	source := types.NewFloat64Series()
	wwma := WWMA(source, window)
	legacy := &indicator.WWMA{IntervalWindow: types.IntervalWindow{Window: window}}

	for _, p := range prices {
		source.PushAndEmit(p)
		legacy.Update(p)
	}

	assert.Equal(t, legacy.Length(), wwma.Length())
	for i := 0; i < wwma.Length(); i++ {
		assert.InDelta(t, legacy.Last(i), wwma.Last(i), 1e-9)
	}
}
