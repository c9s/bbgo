package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_Volatility(t *testing.T) {
	t.Run("hand_computed", func(t *testing.T) {
		var kLines []types.KLine
		for _, c := range []float64{2, 4, 4, 4, 5, 5, 7, 9} {
			kLines = append(kLines, types.KLine{Close: fixedpoint.NewFromFloat(c)})
		}

		stream := &KLineStream{}
		vol := Volatility(stream, 8)

		stream.BackFill(kLines[:7])
		assert.Equal(t, 0, vol.Length(), "no value before the window is filled")

		stream.BackFill(kLines[7:])
		require.Equal(t, 1, vol.Length())
		// population standard deviation of [2, 4, 4, 4, 5, 5, 7, 9]
		assert.InDelta(t, 2.0, vol.Last(0), 1e-9)
	})

	t.Run("compare_with_legacy", func(t *testing.T) {
		const window = 20
		kLines := randomWalkKLines(300, 6)

		stream := &KLineStream{}
		vol := Volatility(stream, window)

		legacy := &indicator.Volatility{IntervalWindow: types.IntervalWindow{Window: window}}
		for i := range kLines {
			stream.BackFill(kLines[i : i+1])
			legacy.CalculateAndUpdate(kLines[:i+1])
		}

		require.Equal(t, len(kLines)-window+1, vol.Length())
		require.Equal(t, legacy.Length(), vol.Length())
		assert.InDeltaSlice(t, []float64(legacy.Values), []float64(vol.Slice), 1e-9)
	})
}
