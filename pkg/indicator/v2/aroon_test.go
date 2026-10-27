package indicatorv2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_Aroon_Basic(t *testing.T) {
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	aroon := Aroon(kLines, 5)

	// Feed 5 candles (0 to 4): Aroon should not emit yet because window=5 requires 6 candles (window + 1)
	highs := []float64{20.0, 22.0, 25.0, 23.0, 21.0, 19.0, 28.0}
	lows := []float64{10.0, 12.0, 15.0, 13.0, 11.0, 9.0, 14.0}

	for i := 0; i < 5; i++ {
		stream.EmitKLineClosed(types.KLine{
			High:    fixedpoint.NewFromFloat(highs[i]),
			Low:     fixedpoint.NewFromFloat(lows[i]),
			EndTime: types.Time(time.Now().Add(time.Duration(i) * time.Minute)),
		})
	}

	assert.Equal(t, 0, aroon.Length(), "should not emit before window + 1 bars")

	// Emit 6th candle (index 5):
	// Window has 6 bars: highs [20, 22, 25, 23, 21, 19], lows [10, 12, 15, 13, 11, 9]
	// Highest high is 25.0 at index 2 (3 bars ago -> periodsAgo = 3)
	// Aroon Up = ((5 - 3) / 5) * 100 = 40.0
	// Lowest low is 9.0 at index 5 (0 bars ago -> periodsAgo = 0)
	// Aroon Down = ((5 - 0) / 5) * 100 = 100.0
	// Aroon Osc = 40.0 - 100.0 = -60.0
	stream.EmitKLineClosed(types.KLine{
		High:    fixedpoint.NewFromFloat(highs[5]),
		Low:     fixedpoint.NewFromFloat(lows[5]),
		EndTime: types.Time(time.Now().Add(5 * time.Minute)),
	})

	assert.Equal(t, 1, aroon.Length())
	assert.InDelta(t, 40.0, aroon.Up.Last(0), 1e-4)
	assert.InDelta(t, 100.0, aroon.Down.Last(0), 1e-4)
	assert.InDelta(t, -60.0, aroon.Oscillator.Last(0), 1e-4)
	assert.InDelta(t, -60.0, aroon.Last(0), 1e-4) // embedded series base returns oscillator

	// Emit 7th candle (index 6):
	// Window has 6 bars: highs [22, 25, 23, 21, 19, 28], lows [12, 15, 13, 11, 9, 14]
	// Highest high is 28.0 at index 6 (0 bars ago -> periodsAgo = 0)
	// Aroon Up = ((5 - 0) / 5) * 100 = 100.0
	// Lowest low is 9.0 at index 5 (1 bar ago -> periodsAgo = 1)
	// Aroon Down = ((5 - 1) / 5) * 100 = 80.0
	// Aroon Osc = 100.0 - 80.0 = 20.0
	stream.EmitKLineClosed(types.KLine{
		High:    fixedpoint.NewFromFloat(highs[6]),
		Low:     fixedpoint.NewFromFloat(lows[6]),
		EndTime: types.Time(time.Now().Add(6 * time.Minute)),
	})

	assert.Equal(t, 2, aroon.Length())
	assert.InDelta(t, 100.0, aroon.Up.Last(0), 1e-4)
	assert.InDelta(t, 80.0, aroon.Down.Last(0), 1e-4)
	assert.InDelta(t, 20.0, aroon.Oscillator.Last(0), 1e-4)
	assert.InDelta(t, 20.0, aroon.Last(0), 1e-4)
}

func Test_Aroon_Ties(t *testing.T) {
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	aroon := Aroon(kLines, 3)

	// Feed candles with equal highest highs at bars 2 and 3
	candles := []struct {
		h, l float64
	}{
		{h: 10.0, l: 5.0},
		{h: 12.0, l: 6.0},
		{h: 20.0, l: 8.0},  // bar 2
		{h: 20.0, l: 10.0}, // bar 3 (tied highest high with bar 2)
	}

	for _, c := range candles {
		stream.EmitKLineClosed(types.KLine{
			High: fixedpoint.NewFromFloat(c.h),
			Low:  fixedpoint.NewFromFloat(c.l),
		})
	}

	// Window is 3 bars lookback (4 bars total: indices 0, 1, 2, 3).
	// High of 20.0 occurred at bar 2 (1 bar ago) and bar 3 (0 bars ago).
	// Per TA-Lib standard, the more recent bar (bar 3, periodsAgo = 0) must be picked.
	// Aroon Up = ((3 - 0) / 3) * 100 = 100.0
	assert.Equal(t, 1, aroon.Length())
	assert.InDelta(t, 100.0, aroon.Up.Last(0), 1e-4)
}

func Test_Aroon_FlatMarket(t *testing.T) {
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	aroon := Aroon(kLines, 4)

	// All candles have identical high and low
	for i := 0; i < 10; i++ {
		stream.EmitKLineClosed(types.KLine{
			High: fixedpoint.NewFromFloat(100.0),
			Low:  fixedpoint.NewFromFloat(50.0),
		})
	}

	// On flat candles, the current bar (0 bars ago) matches the max and min.
	// Aroon Up = 100.0, Aroon Down = 100.0, Osc = 0.0
	assert.Equal(t, 6, aroon.Length())
	for i := 0; i < aroon.Length(); i++ {
		assert.InDelta(t, 100.0, aroon.Up.Last(i), 1e-4)
		assert.InDelta(t, 100.0, aroon.Down.Last(i), 1e-4)
		assert.InDelta(t, 0.0, aroon.Oscillator.Last(i), 1e-4)
	}
}

func Test_Aroon_Callbacks(t *testing.T) {
	stream := &types.StandardStream{}
	kLines := KLines(stream, "", "")
	aroon := Aroon(kLines, 2)

	var receivedUp, receivedDown, receivedOsc []float64
	aroon.OnUpdate(func(up, down, osc float64) {
		receivedUp = append(receivedUp, up)
		receivedDown = append(receivedDown, down)
		receivedOsc = append(receivedOsc, osc)
	})

	bars := [][2]float64{
		{10, 5},
		{12, 6},
		{15, 8}, // bar 2 -> first emission (window=2 requires 3 bars)
		{8, 4},  // bar 3
	}

	for _, b := range bars {
		stream.EmitKLineClosed(types.KLine{
			High: fixedpoint.NewFromFloat(b[0]),
			Low:  fixedpoint.NewFromFloat(b[1]),
		})
	}

	assert.Equal(t, 2, len(receivedUp))
	assert.Equal(t, 2, len(receivedDown))
	assert.Equal(t, 2, len(receivedOsc))
	assert.InDelta(t, receivedUp[0]-receivedDown[0], receivedOsc[0], 1e-4)
	assert.InDelta(t, receivedUp[1]-receivedDown[1], receivedOsc[1], 1e-4)
}
