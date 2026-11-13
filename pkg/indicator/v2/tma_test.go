package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func TestTMA(t *testing.T) {
	source := types.NewFloat64Series()
	// inner and outer windows are (5 + 1) / 2 = 3
	tma := TMA(source, 5)
	for _, v := range []float64{1, 2, 3, 4, 5, 6} {
		source.PushAndEmit(v)
	}

	// inner SMA(3) after warm-up: 2, 3, 4, 5; outer SMA(3) of the last three: (3 + 4 + 5) / 3
	assert.InDelta(t, 5.0, tma.Inner.Last(0), 1e-9)
	assert.InDelta(t, 4.0, tma.Last(0), 1e-9)
	assert.InDelta(t, 3.0, tma.Last(1), 1e-9)
	assert.Equal(t, 6, tma.Length())
}

func TestTMA_CompareLegacy(t *testing.T) {
	prices := randomWalkPrices(100, 6)
	window := 9
	w := (window + 1) / 2

	source := types.NewFloat64Series()
	tma := TMA(source, window)
	legacy := &indicator.TMA{IntervalWindow: types.IntervalWindow{Window: window}}

	for _, p := range prices {
		source.PushAndEmit(p)
		legacy.Update(p)
	}

	// the legacy implementation feeds zeros into the outer SMA during the inner SMA warm-up,
	// so only the values after 2*w-1 inputs are comparable
	for i := 0; i <= len(prices)-(2*w-1); i++ {
		assert.InDelta(t, legacy.Last(i), tma.Last(i), 1e-9)
	}
}
