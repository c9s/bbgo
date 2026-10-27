package indicatorv2

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func TestVWMA(t *testing.T) {
	stream := &KLineStream{}
	vwma := VWMA(stream, 3)
	stream.BackFill([]types.KLine{
		{Close: fixedpoint.NewFromFloat(10), Volume: fixedpoint.NewFromFloat(1)},
		{Close: fixedpoint.NewFromFloat(20), Volume: fixedpoint.NewFromFloat(3)},
		{Close: fixedpoint.NewFromFloat(30), Volume: fixedpoint.NewFromFloat(1)},
		{Close: fixedpoint.NewFromFloat(40), Volume: fixedpoint.NewFromFloat(4)},
	})

	// warm-up values are the VWMA of the available klines
	assert.InDelta(t, 10.0, vwma.Index(3), 1e-9)
	assert.InDelta(t, 70.0/4.0, vwma.Index(2), 1e-9)
	// (10*1 + 20*3 + 30*1) / 5
	assert.InDelta(t, 20.0, vwma.Index(1), 1e-9)
	// (20*3 + 30*1 + 40*4) / 8
	assert.InDelta(t, 31.25, vwma.Last(0), 1e-9)
}

func TestVWMA_CompareLegacy(t *testing.T) {
	prices := randomWalkPrices(100, 4)
	r := rand.New(rand.NewSource(5))
	window := 10

	stream := &KLineStream{}
	vwma := VWMA(stream, window)
	legacy := &indicator.VWMA{IntervalWindow: types.IntervalWindow{Window: window}}

	for _, p := range prices {
		vol := 1 + r.Float64()*10
		k := types.KLine{Close: fixedpoint.NewFromFloat(p), Volume: fixedpoint.NewFromFloat(vol)}
		stream.BackFill([]types.KLine{k})
		legacy.Update(k.Close.Float64(), k.Volume.Float64())
	}

	assert.Equal(t, legacy.Length(), vwma.Length())
	// the legacy implementation emits NaN during the warm-up, so compare the values after the warm-up only
	for i := 0; i <= len(prices)-window; i++ {
		assert.InDelta(t, legacy.Last(i), vwma.Last(i), 1e-9)
	}
}
