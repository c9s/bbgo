package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// Drift is the Brownian Motion's drift factor, which could be used in Monte Carlo Simulations.
//
// Refer: https://tradingview.com/script/aDymGrFx-Drift-Study-Inspired-by-Monte-Carlo-Simulations-with-BM-KL/
//
// In the context of Brownian motion, drift can be measured by calculating the simple moving average (SMA) of the logarithm
// of the price changes of a security over a specified period of time. This SMA can be used to identify the long-term trend
// or bias in the random movement of the security's price. A security with a positive drift is said to be trending upwards,
// while a security with a negative drift is said to be trending downwards. Drift can be used by traders to identify potential
// entry and exit points for trades, or to confirm other technical analysis signals.
// It is typically used in conjunction with other indicators to provide a more comprehensive view of the security's price.
//
// Calculation:
//
//	chng  = log(price / prevPrice)
//	drift = SMA(chng, window) - 0.5 * StdDev(chng, window)^2
//
// The first input only initializes the previous price, and no value is emitted until
// window price changes have been collected.
type DriftStream struct {
	*types.Float64Series

	state driftState
}

// driftState holds the internal calculation state so that it can be cloned for predictions.
type driftState struct {
	window      int
	chng        *types.Queue
	lastValue   float64
	initialized bool
}

func (d *driftState) clone() driftState {
	return driftState{
		window:      d.window,
		chng:        d.chng.Clone(),
		lastValue:   d.lastValue,
		initialized: d.initialized,
	}
}

// update feeds a new price into the state, returns the drift value and whether it is ready.
func (d *driftState) update(value float64) (float64, bool) {
	if !d.initialized {
		d.initialized = true
		d.lastValue = value
		return 0, false
	}

	var chng float64
	if value != 0 {
		chng = math.Log(value / d.lastValue)
		d.lastValue = value
	}

	d.chng.Update(chng)
	if d.chng.Length() < d.window {
		return 0, false
	}

	return d.drift(), true
}

func (d *driftState) drift() float64 {
	stdev := types.Stdev(d.chng, d.window)
	return types.Mean(d.chng, d.window) - stdev*stdev*0.5
}

// zeroPoint assumes that the moving average is a SMA
func (d *driftState) zeroPoint() float64 {
	window := float64(d.window)
	stdev := types.Stdev(d.chng, d.window)
	chng := d.chng.Index(d.window - 1)
	ma := types.Mean(d.chng, d.window)
	return d.lastValue * math.Exp(window*(0.5*stdev*stdev)+chng-ma*window)
}

// Drift creates the drift stream from the given price source (usually the close prices)
func Drift(source types.Float64Source, window int) *DriftStream {
	checkWindow(window)

	s := &DriftStream{
		Float64Series: types.NewFloat64Series(),
		state: driftState{
			window: window,
			chng:   types.NewQueue(window),
		},
	}

	s.Subscribe(source, func(v float64) {
		if drift, ok := s.state.update(v); ok {
			s.PushAndEmit(drift)
			s.Truncate()
		}
	})
	return s
}

// ZeroPoint returns the price that would bring the drift to the zero point
// (assuming that the moving average is a SMA).
func (s *DriftStream) ZeroPoint() float64 {
	return s.state.zeroPoint()
}

// TestUpdate returns the drift value as if the given price were pushed,
// without changing the state of the stream.
// It returns 0 if the drift would not be ready yet.
func (s *DriftStream) TestUpdate(value float64) float64 {
	st := s.state.clone()
	drift, _ := st.update(value)
	return drift
}

func (s *DriftStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}
