package indicatorv2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestAroon_Uptrend(t *testing.T) {
	stream := &KLineStream{}
	aroon := Aroon(stream, 5)

	now := time.Now()
	// Strictly increasing highs and lows
	for i := 1; i <= 10; i++ {
		stream.EmitUpdate(types.KLine{
			High:    fixedpoint.NewFromFloat(float64(i * 10)),
			Low:     fixedpoint.NewFromFloat(float64(i*10 - 5)),
			EndTime: types.Time(now.Add(time.Duration(i) * time.Minute)),
		})
	}

	// In sustained uptrend:
	// Highest high is at the current bar (periods since = 0): Aroon Up = 100.
	// Lowest low was 5 periods ago (oldest bar in 5+1 window): Aroon Down = ((5 - 5) / 5) * 100 = 0.
	// Oscillator = 100 - 0 = +100.
	assert.Equal(t, 100.0, aroon.LastUp())
	assert.Equal(t, 0.0, aroon.LastDown())
	assert.Equal(t, 100.0, aroon.LastOscillator())
	assert.Equal(t, 100.0, aroon.Last(0))
}

func TestAroon_Downtrend(t *testing.T) {
	stream := &KLineStream{}
	aroon := Aroon(stream, 5)

	now := time.Now()
	// Strictly decreasing highs and lows
	for i := 10; i >= 1; i-- {
		stream.EmitUpdate(types.KLine{
			High:    fixedpoint.NewFromFloat(float64(i * 10)),
			Low:     fixedpoint.NewFromFloat(float64(i*10 - 5)),
			EndTime: types.Time(now.Add(time.Duration(10-i) * time.Minute)),
		})
	}

	// In sustained downtrend:
	// Highest high was 5 periods ago: Aroon Up = ((5 - 5) / 5) * 100 = 0.
	// Lowest low is at current bar: Aroon Down = 100.
	// Oscillator = 0 - 100 = -100.
	assert.Equal(t, 0.0, aroon.LastUp())
	assert.Equal(t, 100.0, aroon.LastDown())
	assert.Equal(t, -100.0, aroon.LastOscillator())
	assert.Equal(t, -100.0, aroon.Last(0))
}

func TestAroon_Flat(t *testing.T) {
	stream := &KLineStream{}
	aroon := Aroon(stream, 5)

	now := time.Now()
	for i := 0; i < 10; i++ {
		stream.EmitUpdate(types.KLine{
			High:    fixedpoint.NewFromFloat(100.0),
			Low:     fixedpoint.NewFromFloat(90.0),
			EndTime: types.Time(now.Add(time.Duration(i) * time.Minute)),
		})
	}

	// In flat market, most recent bar is selected as extreme:
	// Highest high periods since = 0 -> Up = 100.
	// Lowest low periods since = 0 -> Down = 100.
	// Oscillator = 100 - 100 = 0.
	assert.Equal(t, 100.0, aroon.LastUp())
	assert.Equal(t, 100.0, aroon.LastDown())
	assert.Equal(t, 0.0, aroon.LastOscillator())
}

func TestAroon_SpecificKnownValues(t *testing.T) {
	stream := &KLineStream{}
	aroon := Aroon(stream, 4)

	highs := []float64{10, 20, 15, 12, 18}
	lows := []float64{5, 6, 7, 8, 4}

	now := time.Now()
	for i := 0; i < len(highs); i++ {
		stream.EmitUpdate(types.KLine{
			High:    fixedpoint.NewFromFloat(highs[i]),
			Low:     fixedpoint.NewFromFloat(lows[i]),
			EndTime: types.Time(now.Add(time.Duration(i) * time.Minute)),
		})
	}

	// At bar 4:
	// Window = 4, lookback includes 5 bars (bar 0 to bar 4).
	// Highest high is 20 at bar 1: periods since = 4 - 1 = 3.
	// Aroon Up = ((4 - 3) / 4) * 100 = 25.0%.
	// Lowest low is 4 at bar 4: periods since = 4 - 4 = 0.
	// Aroon Down = ((4 - 0) / 4) * 100 = 100.0%.
	// Aroon Oscillator = 25.0 - 100.0 = -75.0.
	assert.Equal(t, 25.0, aroon.LastUp())
	assert.Equal(t, 100.0, aroon.LastDown())
	assert.Equal(t, -75.0, aroon.LastOscillator())
}

func TestAroon_SeriesAndCallbacks(t *testing.T) {
	stream := &KLineStream{}
	aroon := Aroon(stream, 4)

	var callbackUpdates [][2]float64
	aroon.OnUpdate(func(up, down float64) {
		callbackUpdates = append(callbackUpdates, [2]float64{up, down})
	})

	var oscUpdates []float64
	aroon.Oscillator.OnUpdate(func(osc float64) {
		oscUpdates = append(oscUpdates, osc)
	})

	highs := []float64{10, 20, 15, 12, 18}
	lows := []float64{5, 6, 7, 8, 4}

	now := time.Now()
	for i := 0; i < len(highs); i++ {
		stream.EmitUpdate(types.KLine{
			High:    fixedpoint.NewFromFloat(highs[i]),
			Low:     fixedpoint.NewFromFloat(lows[i]),
			EndTime: types.Time(now.Add(time.Duration(i) * time.Minute)),
		})
	}

	upSeries := aroon.AroonUp()
	downSeries := aroon.AroonDown()
	oscSeries := aroon.AroonOscillator()

	assert.Equal(t, 5, upSeries.Length())
	assert.Equal(t, 5, downSeries.Length())
	assert.Equal(t, 5, oscSeries.Length())
	assert.Equal(t, 5, aroon.Length())

	assert.Equal(t, 25.0, upSeries.Last(0))
	assert.Equal(t, 100.0, downSeries.Last(0))
	assert.Equal(t, -75.0, oscSeries.Last(0))
	assert.Equal(t, -75.0, aroon.Last(0))
	assert.Equal(t, -75.0, aroon.Index(0))

	assert.Equal(t, 5, len(callbackUpdates))
	assert.Equal(t, [2]float64{25.0, 100.0}, callbackUpdates[len(callbackUpdates)-1])
	assert.Equal(t, 5, len(oscUpdates))
	assert.Equal(t, -75.0, oscUpdates[len(oscUpdates)-1])
}

func TestAroon_BackFill(t *testing.T) {
	stream := &KLineStream{}
	aroon := Aroon(stream, 14)
	stream.BackFill(randomWalkKLines(50, 1))

	assert.Equal(t, 50, aroon.Length())
	assert.Equal(t, aroon.Oscillator.Last(0), aroon.Last(0))
}

func TestAroon_ZeroWindowPanic(t *testing.T) {
	stream := &KLineStream{}
	assert.PanicsWithValue(t, "window can not be zero", func() {
		Aroon(stream, 0)
	})
}
