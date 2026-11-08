package indicator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestAroon_Uptrend(t *testing.T) {
	aroon := &Aroon{
		IntervalWindow: types.IntervalWindow{Window: 5},
	}

	// Strictly increasing highs and lows
	for i := 1; i <= 10; i++ {
		aroon.Update(float64(i*10), float64(i*10-5))
	}

	// In a sustained uptrend:
	// Highest high is at the current bar (periods since = 0): Aroon Up = 100.
	// Lowest low was 5 periods ago (oldest bar in window): Aroon Down = ((5 - 5) / 5) * 100 = 0.
	// Oscillator = 100 - 0 = +100.
	assert.Equal(t, 100.0, aroon.LastUp())
	assert.Equal(t, 0.0, aroon.LastDown())
	assert.Equal(t, 100.0, aroon.LastOscillator())
}

func TestAroon_Downtrend(t *testing.T) {
	aroon := &Aroon{
		IntervalWindow: types.IntervalWindow{Window: 5},
	}

	// Strictly decreasing highs and lows
	for i := 10; i >= 1; i-- {
		aroon.Update(float64(i*10), float64(i*10-5))
	}

	// In a sustained downtrend:
	// Highest high was 5 periods ago (oldest bar in window): Aroon Up = ((5 - 5) / 5) * 100 = 0.
	// Lowest low is at the current bar (periods since = 0): Aroon Down = 100.
	// Oscillator = 0 - 100 = -100.
	assert.Equal(t, 0.0, aroon.LastUp())
	assert.Equal(t, 100.0, aroon.LastDown())
	assert.Equal(t, -100.0, aroon.LastOscillator())
}

func TestAroon_Flat(t *testing.T) {
	aroon := &Aroon{
		IntervalWindow: types.IntervalWindow{Window: 5},
	}

	// Completely flat market: highs and lows identical across bars
	for i := 0; i < 10; i++ {
		aroon.Update(100.0, 90.0)
	}

	// In flat market, the most recent bar is selected as the extreme:
	// Highest high periods since = 0 -> Up = 100.
	// Lowest low periods since = 0 -> Down = 100.
	// Oscillator = 100 - 100 = 0.
	assert.Equal(t, 100.0, aroon.LastUp())
	assert.Equal(t, 100.0, aroon.LastDown())
	assert.Equal(t, 0.0, aroon.LastOscillator())
}

func TestAroon_SpecificKnownValues(t *testing.T) {
	aroon := &Aroon{
		IntervalWindow: types.IntervalWindow{Window: 4},
	}

	// Data points:
	// Bar 0: high 10, low 5
	// Bar 1: high 20, low 6 (highest high in window of 4 + 1 bars)
	// Bar 2: high 15, low 7
	// Bar 3: high 12, low 8
	// Bar 4: high 18, low 4 (lowest low in window of 4 + 1 bars)
	highs := []float64{10, 20, 15, 12, 18}
	lows := []float64{5, 6, 7, 8, 4}

	for i := 0; i < len(highs); i++ {
		aroon.Update(highs[i], lows[i])
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

func TestAroon_SeriesExtend(t *testing.T) {
	aroon := &Aroon{
		IntervalWindow: types.IntervalWindow{Window: 4},
	}

	highs := []float64{10, 20, 15, 12, 18}
	lows := []float64{5, 6, 7, 8, 4}

	for i := 0; i < len(highs); i++ {
		aroon.Update(highs[i], lows[i])
	}

	upSeries := aroon.AroonUp()
	downSeries := aroon.AroonDown()
	oscSeries := aroon.AroonOscillator()

	assert.Equal(t, 5, upSeries.Length())
	assert.Equal(t, 5, downSeries.Length())
	assert.Equal(t, 5, oscSeries.Length())

	assert.Equal(t, 25.0, upSeries.Last(0))
	assert.Equal(t, 100.0, downSeries.Last(0))
	assert.Equal(t, -75.0, oscSeries.Last(0))

	assert.Equal(t, 25.0, aroon.GetUp().Last(0))
	assert.Equal(t, 100.0, aroon.GetDown().Last(0))
	assert.Equal(t, -75.0, aroon.GetOscillator().Last(0))

	assert.Equal(t, -75.0, aroon.Last(0))
	assert.Equal(t, -75.0, aroon.Index(0))
	assert.Equal(t, 5, aroon.Length())
}

func TestAroon_PushK_And_Callbacks(t *testing.T) {
	aroon := &Aroon{
		IntervalWindow: types.IntervalWindow{Window: 3},
	}

	var callbackUpdates [][2]float64
	aroon.OnUpdate(func(up float64, down float64) {
		callbackUpdates = append(callbackUpdates, [2]float64{up, down})
	})

	now := time.Now()
	klines := []types.KLine{
		{
			High:    fixedpoint.NewFromFloat(10),
			Low:     fixedpoint.NewFromFloat(5),
			EndTime: types.Time(now.Add(1 * time.Minute)),
		},
		{
			High:    fixedpoint.NewFromFloat(15),
			Low:     fixedpoint.NewFromFloat(7),
			EndTime: types.Time(now.Add(2 * time.Minute)),
		},
		{
			High:    fixedpoint.NewFromFloat(12),
			Low:     fixedpoint.NewFromFloat(6),
			EndTime: types.Time(now.Add(3 * time.Minute)),
		},
	}

	for _, k := range klines {
		aroon.PushK(k)
	}

	assert.Equal(t, len(klines), len(callbackUpdates))
	assert.Equal(t, aroon.LastUp(), callbackUpdates[len(callbackUpdates)-1][0])
	assert.Equal(t, aroon.LastDown(), callbackUpdates[len(callbackUpdates)-1][1])
}

func TestAroon_EdgeCases(t *testing.T) {
	// Empty indicator
	emptyAroon := &Aroon{
		IntervalWindow: types.IntervalWindow{Window: 14},
	}
	up, down := emptyAroon.Calculate()
	assert.Equal(t, 0.0, up)
	assert.Equal(t, 0.0, down)
	assert.Equal(t, 0.0, emptyAroon.LastUp())
	assert.Equal(t, 0.0, emptyAroon.LastDown())
	assert.Equal(t, 0.0, emptyAroon.LastOscillator())

	// Window <= 0 defaults to 14 upon update
	zeroWindowAroon := &Aroon{
		IntervalWindow: types.IntervalWindow{Window: 0},
	}
	zeroWindowAroon.Update(50.0, 40.0)
	assert.Equal(t, 14, zeroWindowAroon.Window)
	assert.Equal(t, 100.0, zeroWindowAroon.LastUp())
	assert.Equal(t, 100.0, zeroWindowAroon.LastDown())
	assert.Equal(t, 0.0, zeroWindowAroon.LastOscillator())
}
