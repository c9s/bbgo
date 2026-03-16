package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func vpKLine(close, volume float64) types.KLine {
	p := fixedpoint.NewFromFloat(close)
	return types.KLine{
		Open:   p,
		High:   p,
		Low:    p,
		Close:  p,
		Volume: fixedpoint.NewFromFloat(volume),
	}
}

func vpOHLCKLine(open, high, low, close, volume float64) types.KLine {
	return types.KLine{
		Open:   fixedpoint.NewFromFloat(open),
		High:   fixedpoint.NewFromFloat(high),
		Low:    fixedpoint.NewFromFloat(low),
		Close:  fixedpoint.NewFromFloat(close),
		Volume: fixedpoint.NewFromFloat(volume),
	}
}

func TestInvalidDelta(t *testing.T) {
	t.Run("ZeroDelta", func(t *testing.T) {
		stream := &KLineStream{}
		assert.Panics(t, func() {
			NewFixedWindowVolumeProfile(stream, 3, 0)
		})
	})
	t.Run("NegativeDelta", func(t *testing.T) {
		stream := &KLineStream{}
		assert.Panics(t, func() {
			NewFixedWindowVolumeProfile(stream, 3, -1.0)
		})
	})
}

func TestFixedWindowVolumeProfile_WindowCompletion(t *testing.T) {
	stream := &KLineStream{}
	vp := NewFixedWindowVolumeProfile(stream, 3, 1.0)

	var emittedProfile map[float64]float64
	vp.OnReset(func(p map[float64]float64) { emittedProfile = p })

	stream.EmitUpdate(vpKLine(1.0, 10.0))
	stream.EmitUpdate(vpKLine(2.0, 30.0))
	assert.Nil(t, emittedProfile, "window should not be complete yet")

	stream.EmitUpdate(vpKLine(3.0, 5.0))
	assert.NotNil(t, emittedProfile, "window should have completed")
	assert.Equal(t, 10.0, emittedProfile[1.0])
	assert.Equal(t, 30.0, emittedProfile[2.0])
	assert.Equal(t, 5.0, emittedProfile[3.0])
}

func Test_PointOfControl(t *testing.T) {
	t.Run("BasicPoC", func(t *testing.T) {
		stream := &KLineStream{}
		// large window so it never completes during the test
		vp := NewFixedWindowVolumeProfile(stream, 100, 1.0)

		stream.EmitUpdate(vpKLine(1.0, 5.0))
		stream.EmitUpdate(vpKLine(2.0, 10.0))
		stream.EmitUpdate(vpKLine(3.0, 30.0))

		poc, vol := vp.PointOfControl()
		assert.Equal(t, 3.0, poc)
		assert.Equal(t, 30.0, vol)
	})

	t.Run("AccumulatesVolume", func(t *testing.T) {
		stream := &KLineStream{}
		vp := NewFixedWindowVolumeProfile(stream, 100, 1.0)

		// two klines at the same price level should accumulate
		stream.EmitUpdate(vpKLine(2.0, 10.0))
		stream.EmitUpdate(vpKLine(2.0, 20.0))
		stream.EmitUpdate(vpKLine(3.0, 25.0))

		poc, vol := vp.PointOfControl()
		assert.Equal(t, 2.0, poc)
		assert.Equal(t, 30.0, vol)
	})

	t.Run("PushedToSeries", func(t *testing.T) {
		stream := &KLineStream{}
		vp := NewFixedWindowVolumeProfile(stream, 3, 1.0)

		stream.EmitUpdate(vpKLine(1.0, 5.0))
		stream.EmitUpdate(vpKLine(2.0, 10.0))
		stream.EmitUpdate(vpKLine(3.0, 30.0))
		// window complete: PoC = 3.0
		assert.Equal(t, 3.0, vp.Last(0))
		assert.Equal(t, 1, vp.Length())
	})

	t.Run("MultipleWindows", func(t *testing.T) {
		stream := &KLineStream{}
		vp := NewFixedWindowVolumeProfile(stream, 2, 1.0)

		// first window: price 2 has higher volume → PoC = 2.0
		stream.EmitUpdate(vpKLine(1.0, 10.0))
		stream.EmitUpdate(vpKLine(2.0, 20.0))
		assert.Equal(t, 2.0, vp.Last(0))

		// second window: price 1 has higher volume → PoC = 1.0
		stream.EmitUpdate(vpKLine(3.0, 5.0))
		stream.EmitUpdate(vpKLine(1.0, 15.0))
		assert.Equal(t, 1.0, vp.Last(0))

		assert.Equal(t, 2, vp.Length())
	})

	t.Run("AboveEqual", func(t *testing.T) {
		stream := &KLineStream{}
		// large window so profile never resets
		vp := NewFixedWindowVolumeProfile(stream, 100, 1.0)

		// profile: price 1→vol 5, price 2→vol 10, price 3→vol 30
		stream.EmitUpdate(vpKLine(1.0, 5.0))
		stream.EmitUpdate(vpKLine(2.0, 10.0))
		stream.EmitUpdate(vpKLine(3.0, 30.0))

		// at or above 2.0: buckets 2 and 3 → PoC at price 3 (vol 30)
		poc, vol := vp.PointOfControlAboveEqual(2.0)
		assert.Equal(t, 3.0, poc)
		assert.Equal(t, 30.0, vol)

		// at or above 4.0: nothing above max → returns (0, 0)
		poc, vol = vp.PointOfControlAboveEqual(4.0)
		assert.Equal(t, 0.0, poc)
		assert.Equal(t, 0.0, vol)
	})

	t.Run("BelowEqual", func(t *testing.T) {
		stream := &KLineStream{}
		vp := NewFixedWindowVolumeProfile(stream, 100, 1.0)

		// profile: price 1→vol 5, price 2→vol 30, price 3→vol 40
		stream.EmitUpdate(vpKLine(1.0, 5.0))
		stream.EmitUpdate(vpKLine(2.0, 30.0))
		stream.EmitUpdate(vpKLine(3.0, 40.0))

		// at or below 2.0: buckets 1 and 2 → PoC at price 2 (vol 30)
		poc, vol := vp.PointOfControlBelowEqual(2.0)
		assert.Equal(t, 2.0, poc)
		assert.Equal(t, 30.0, vol)

		// at or below 0.0: below min → returns (0, 0)
		poc, vol = vp.PointOfControlBelowEqual(0.0)
		assert.Equal(t, 0.0, poc)
		assert.Equal(t, 0.0, vol)
	})
}

func TestFixedWindowVolumeProfile_ResetClearsProfile(t *testing.T) {
	stream := &KLineStream{}
	vp := NewFixedWindowVolumeProfile(stream, 2, 1.0)

	stream.EmitUpdate(vpKLine(1.0, 10.0))
	stream.EmitUpdate(vpKLine(2.0, 20.0))
	// window complete and reset

	// new kline starts a fresh profile
	stream.EmitUpdate(vpKLine(5.0, 50.0))
	poc, vol := vp.PointOfControl()
	assert.Equal(t, 5.0, poc)
	assert.Equal(t, 50.0, vol)
}

func TestFixedWindowVolumeProfile_ZeroVolumeSkipped(t *testing.T) {
	stream := &KLineStream{}
	vp := NewFixedWindowVolumeProfile(stream, 2, 1.0)

	var resetFired bool
	vp.OnReset(func(_ map[float64]float64) { resetFired = true })

	stream.EmitUpdate(vpKLine(1.0, 0.0)) // skipped
	stream.EmitUpdate(vpKLine(2.0, 0.0)) // skipped
	assert.False(t, resetFired, "zero-volume klines should not count toward the window")

	stream.EmitUpdate(vpKLine(3.0, 10.0))
	stream.EmitUpdate(vpKLine(4.0, 20.0))
	assert.True(t, resetFired)
}

func TestFixedWindowVolumeProfile_UseAvgOHLC(t *testing.T) {
	stream := &KLineStream{}
	vp := NewFixedWindowVolumeProfile(stream, 1, 1.0)
	vp.UseAvgOHLC()

	var emittedProfile map[float64]float64
	vp.OnReset(func(p map[float64]float64) { emittedProfile = p })

	// O=0, H=4, L=0, C=4 → avgOHLC = (0+4+0+4)/4 = 2.0 → bucket 2
	stream.EmitUpdate(vpOHLCKLine(0.0, 4.0, 0.0, 4.0, 10.0))

	assert.Equal(t, 10.0, emittedProfile[2.0], "volume should be in the avg-OHLC bucket")
	assert.Equal(t, 0.0, emittedProfile[4.0], "close-only bucket should be empty")
}
