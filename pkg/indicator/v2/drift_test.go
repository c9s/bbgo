package indicatorv2

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

var driftTestPrices = []float64{1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 4, 1, 2, 3, 4, 5, 6, 7, 8, 9, 4, 1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 4, 1, 2, 3, 4, 5, 6, 7, 8, 9}

// randomWalkPrices generates deterministic positive prices for comparing against the legacy implementation
func randomWalkPrices(n int, seed int64) []float64 {
	r := rand.New(rand.NewSource(seed))
	prices := make([]float64, n)
	p := 100.0
	for i := range prices {
		p *= 1 + (r.Float64()-0.5)*0.04
		prices[i] = p
	}
	return prices
}

func TestDrift(t *testing.T) {
	// ported from the legacy Test_Drift
	source := types.NewFloat64Series()
	drift := Drift(source, 3)
	for _, p := range driftTestPrices {
		source.PushAndEmit(p)
	}

	assert.Equal(t, 47, drift.Length())
	for _, v := range drift.Slice {
		assert.LessOrEqual(t, v, 1.0)
	}
}

func TestDrift_CompareLegacy(t *testing.T) {
	prices := randomWalkPrices(200, 1)
	window := 14

	source := types.NewFloat64Series()
	drift := Drift(source, window)
	legacy := &indicator.Drift{IntervalWindow: types.IntervalWindow{Window: window}}

	for _, p := range prices {
		// the prediction must match the real update and must not mutate the state
		predicted := drift.TestUpdate(p)
		n := drift.Length()

		source.PushAndEmit(p)
		legacy.Update(p)

		if drift.Length() > n {
			assert.InDelta(t, drift.Last(0), predicted, 1e-12)
		}
	}

	assert.Equal(t, legacy.Length(), drift.Length())
	for i := 0; i < drift.Length(); i++ {
		assert.InDelta(t, legacy.Last(i), drift.Last(i), 1e-12)
	}

	assert.InDelta(t, legacy.ZeroPoint(), drift.ZeroPoint(), 1e-9)
	assert.InDelta(t, legacy.TestUpdate(101.0).Last(0), drift.TestUpdate(101.0), 1e-12)
}

func TestDrift_ClosePrices(t *testing.T) {
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	drift := Drift(ClosePrices(kLines), 3)

	for _, p := range driftTestPrices {
		stream.EmitKLineClosed(types.KLine{Close: fixedpoint.NewFromFloat(p)})
	}

	assert.Equal(t, 47, drift.Length())
	assert.False(t, math.IsNaN(drift.Last(0)))
}
