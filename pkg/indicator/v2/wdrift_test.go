package indicatorv2

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func TestWeightedDrift(t *testing.T) {
	// ported from the legacy Test_WDrift
	var kLines []types.KLine
	for _, p := range driftTestPrices {
		kLines = append(kLines, types.KLine{Close: fixedpoint.NewFromFloat(p), Volume: fixedpoint.One})
	}

	stream := &KLineStream{}
	drift := WeightedDrift(stream, 3)
	stream.BackFill(kLines)

	assert.Equal(t, 47, drift.Length())
	for _, v := range drift.Slice {
		assert.LessOrEqual(t, v, 1.0)
	}
}

func TestWeightedDrift_CompareLegacy(t *testing.T) {
	prices := randomWalkPrices(200, 2)
	r := rand.New(rand.NewSource(3))
	window := 10

	var kLines []types.KLine
	for i, p := range prices {
		vol := 1 + r.Float64()*5
		if i%17 == 5 {
			vol = 0 // zero volume klines only update the last value
		}
		kLines = append(kLines, types.KLine{Close: fixedpoint.NewFromFloat(p), Volume: fixedpoint.NewFromFloat(vol)})
	}

	stream := &KLineStream{}
	drift := WeightedDrift(stream, window)
	legacy := &indicator.WeightedDrift{IntervalWindow: types.IntervalWindow{Window: window}}

	for _, k := range kLines {
		predicted := drift.TestUpdate(k.Close.Float64(), k.Volume.Float64())
		n := drift.Length()

		stream.BackFill([]types.KLine{k})
		legacy.PushK(k)

		if drift.Length() > n {
			assert.InDelta(t, drift.Last(0), predicted, 1e-12)
		}
	}

	assert.Equal(t, legacy.Length(), drift.Length())
	for i := 0; i < drift.Length(); i++ {
		assert.InDelta(t, legacy.Last(i), drift.Last(i), 1e-12)
	}

	assert.InDelta(t, legacy.ZeroPoint(), drift.ZeroPoint(), 1e-9)
	assert.InDelta(t, legacy.TestUpdate(101.0, 3.0).Last(0), drift.TestUpdate(101.0, 3.0), 1e-12)
}
