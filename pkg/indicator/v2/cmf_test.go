package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func buildCMFKLine(high, low, closePrice, volume float64) types.KLine {
	return types.KLine{
		High:   fixedpoint.NewFromFloat(high),
		Low:    fixedpoint.NewFromFloat(low),
		Close:  fixedpoint.NewFromFloat(closePrice),
		Volume: fixedpoint.NewFromFloat(volume),
	}
}

func Test_CMF2(t *testing.T) {
	newStream := func(window int) (*types.StandardStream, *KLineStream, *CMFStream) {
		stream := &types.StandardStream{}
		kLines := KLines(stream, "", "")
		cmf := CMF2(kLines, window)
		return stream, kLines, cmf
	}

	t.Run("normal_calculation", func(t *testing.T) {
		// Bar 1: H=10, L=5, C=7.5 (mid), V=100 => MFM = 0, MFV = 0
		// Bar 2: H=15, L=5, C=15 (high), V=200 => MFM = 1, MFV = 200
		// Bar 3: H=12, L=8, C=8 (low), V=100 => MFM = -1, MFV = -100
		// Window = 3: sumMFV = 100, sumVol = 400 => CMF = 100 / 400 = 0.25
		stream, _, cmf := newStream(3)

		stream.EmitKLineClosed(buildCMFKLine(10, 5, 7.5, 100))
		assert.Equal(t, 0, cmf.Length(), "should not emit before window is filled")

		stream.EmitKLineClosed(buildCMFKLine(15, 5, 15, 200))
		assert.Equal(t, 0, cmf.Length(), "should not emit before window is filled")

		stream.EmitKLineClosed(buildCMFKLine(12, 8, 8, 100))
		assert.Equal(t, 1, cmf.Length())
		assert.InDelta(t, 0.25, cmf.Last(0), 1e-9)
		assert.InDelta(t, 0.25, cmf.Index(0), 1e-9)
	})

	t.Run("rolling_window", func(t *testing.T) {
		stream, _, cmf := newStream(3)

		stream.EmitKLineClosed(buildCMFKLine(10, 5, 7.5, 100))
		stream.EmitKLineClosed(buildCMFKLine(15, 5, 15, 200))
		stream.EmitKLineClosed(buildCMFKLine(12, 8, 8, 100))

		// Bar 4: H=20, L=10, C=20 (high), V=100 => MFM = 1, MFV = 100
		// Window covers bars 2, 3, 4:
		// sumMFV = 200 + (-100) + 100 = 200
		// sumVol = 200 + 100 + 100 = 400
		// CMF = 200 / 400 = 0.5
		stream.EmitKLineClosed(buildCMFKLine(20, 10, 20, 100))
		assert.Equal(t, 2, cmf.Length())
		assert.InDelta(t, 0.5, cmf.Last(0), 1e-9)
		assert.InDelta(t, 0.25, cmf.Last(1), 1e-9)
	})

	t.Run("zero_range_candle", func(t *testing.T) {
		stream, _, cmf := newStream(2)

		stream.EmitKLineClosed(buildCMFKLine(10, 10, 10, 100))
		assert.Equal(t, 0, cmf.Length())

		stream.EmitKLineClosed(buildCMFKLine(10, 10, 10, 100))
		assert.Equal(t, 1, cmf.Length())
		assert.InDelta(t, 0.0, cmf.Last(0), 1e-9)
	})

	t.Run("zero_volume", func(t *testing.T) {
		stream, _, cmf := newStream(2)

		stream.EmitKLineClosed(buildCMFKLine(15, 5, 15, 0))
		stream.EmitKLineClosed(buildCMFKLine(15, 5, 15, 0))
		assert.Equal(t, 1, cmf.Length())
		assert.InDelta(t, 0.0, cmf.Last(0), 1e-9)
	})

	t.Run("insufficient_samples", func(t *testing.T) {
		stream, _, cmf := newStream(3)

		stream.EmitKLineClosed(buildCMFKLine(10, 5, 10, 100))
		stream.EmitKLineClosed(buildCMFKLine(10, 5, 10, 100))
		assert.Equal(t, 0, cmf.Length(), "should not emit before window is filled")
	})

	t.Run("update_callbacks", func(t *testing.T) {
		stream, _, cmf := newStream(2)

		var callbacks []float64
		cmf.OnUpdate(func(v float64) {
			callbacks = append(callbacks, v)
		})

		// H=10, L=5, C=10, V=100 => MFM = 1, MFV = 100
		// Window = 2: CMF = (100 + 100) / (100 + 100) = 1.0
		stream.EmitKLineClosed(buildCMFKLine(10, 5, 10, 100))
		assert.Equal(t, 0, len(callbacks))

		stream.EmitKLineClosed(buildCMFKLine(10, 5, 10, 100))
		assert.Equal(t, 1, len(callbacks))
		assert.InDelta(t, 1.0, callbacks[0], 1e-9)
	})

	t.Run("truncate", func(t *testing.T) {
		stream, _, cmf := newStream(1)
		for i := 0; i < MaxSliceSize+10; i++ {
			stream.EmitKLineClosed(buildCMFKLine(10, 5, 10, 100))
		}
		assert.Equal(t, MaxSliceSize+10, cmf.Length())
		cmf.Truncate()
		assert.Equal(t, TruncateSize, cmf.Length())
	})

	t.Run("invalid_window", func(t *testing.T) {
		stream := &types.StandardStream{}
		kLines := KLines(stream, "", "")
		assert.Panics(t, func() {
			CMF2(kLines, 0)
		})
	})
}
