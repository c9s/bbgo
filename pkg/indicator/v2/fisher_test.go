package indicatorv2

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_FisherTransform(t *testing.T) {
	t.Run("hand computed", func(t *testing.T) {
		source := types.NewFloat64Series()
		fisher := FisherTransform(source, 3)
		for _, v := range []float64{1, 1, 3, 2} {
			source.PushAndEmit(v)
		}

		// [1]: highest == lowest -> 0
		// [1, 1]: highest == lowest -> 0
		// [1, 1, 3]: x = 1 -> 0.9999
		// [1, 3, 2]: x = 0
		assert.Equal(t, 4, fisher.Length())
		assert.Equal(t, 0.0, fisher.Index(3))
		assert.Equal(t, 0.0, fisher.Index(2))
		assert.InDelta(t, 0.5*math.Log(1.9999/0.0001), fisher.Index(1), 1e-9)
		assert.InDelta(t, 0.0, fisher.Last(0), 1e-9)
	})

	t.Run("compare with legacy", func(t *testing.T) {
		klines := loadETHUSDT5mKLines(t)

		for _, window := range []int{5, 10, 30} {
			legacy := &indicator.FisherTransform{IntervalWindow: types.IntervalWindow{Window: window}}

			stream := &KLineStream{}
			fisher := FisherTransform(ClosePrices(stream), window)
			stream.BackFill(klines)

			for _, k := range klines {
				legacy.Update(k.Close.Float64())
			}

			assert.Equal(t, len(klines), fisher.Length())
			// legacy truncates its values to a shorter length; compare the overlapping tail
			for i := 0; i < legacy.Length(); i++ {
				assert.InDelta(t, legacy.Index(i), fisher.Index(i), 1e-9, "window %d index %d", window, i)
			}
		}
	})
}
