package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_LinReg(t *testing.T) {
	t.Run("linear input", func(t *testing.T) {
		// a perfectly linear input y = 2x + 10 has a regression slope of 2
		source := types.NewFloat64Series()
		lr := LinReg(source, 5)
		for i := 0; i < 10; i++ {
			source.PushAndEmit(2*float64(i) + 10)
		}

		assert.Equal(t, 10, lr.Length())
		assert.Equal(t, 10, lr.ValueRatios.Length())
		for i := 6; i < 10; i++ {
			// warm-up values are 0
			assert.Equal(t, 0.0, lr.Index(i))
			assert.Equal(t, 0.0, lr.IndexRatio(i))
		}
		assert.InDelta(t, 2.0, lr.Last(0), 1e-9)
		assert.InDelta(t, 2.0/28.0, lr.LastRatio(), 1e-9)
	})

	t.Run("compare with legacy", func(t *testing.T) {
		klines := loadETHUSDT5mKLines(t)

		for _, window := range []int{5, 20, 99} {
			legacy := &indicator.LinReg{IntervalWindow: types.IntervalWindow{Window: window}}
			for _, k := range klines {
				legacy.Update(k)
			}

			stream := &KLineStream{}
			lr := LinReg(ClosePrices(stream), window)
			stream.BackFill(klines)

			assert.Equal(t, legacy.Length(), lr.Length())
			assert.Equal(t, legacy.LengthRatio(), lr.ValueRatios.Length())
			for i := 0; i < legacy.Length(); i++ {
				assert.InDelta(t, legacy.Index(i), lr.Index(i), 1e-9, "window %d index %d", window, i)
				assert.InDelta(t, legacy.IndexRatio(i), lr.IndexRatio(i), 1e-12, "window %d index %d", window, i)
			}
		}
	})
}
