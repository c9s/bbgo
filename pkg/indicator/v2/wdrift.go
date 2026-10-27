package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// WeightedDrift is the volume weighted Brownian Motion's drift factor,
// which could be used in Monte Carlo Simulations.
//
// Refer: https://tradingview.com/script/aDymGrFx-Drift-Study-Inspired-by-Monte-Carlo-Simulations-with-BM-KL/
//
// It works like Drift, but each log price change is weighted by the kline volume:
// the change is normalized by weight/base (base = the lowest weight in the window),
// and pushed int(weight/base) times into the change window, so that heavier
// candles take more slots of the window.
//
// KLines with zero volume only update the previous price.
type WeightedDriftStream struct {
	*types.Float64Series

	state weightedDriftState
}

type weightedDriftState struct {
	window      int
	chng        *types.Queue
	weight      *types.Queue
	lastValue   float64
	initialized bool
}

func (d *weightedDriftState) clone() weightedDriftState {
	return weightedDriftState{
		window:      d.window,
		chng:        d.chng.Clone(),
		weight:      d.weight.Clone(),
		lastValue:   d.lastValue,
		initialized: d.initialized,
	}
}

func (d *weightedDriftState) update(value, weight float64) (float64, bool) {
	if weight == 0 {
		d.lastValue = value
		return 0, false
	}

	if !d.initialized {
		d.initialized = true
		d.lastValue = value
		d.weight.Update(weight)
		return 0, false
	}

	d.weight.Update(weight)
	base := d.weight.Lowest(d.window)
	multiplier := int(weight / base)

	var chng float64
	if value != 0 {
		chng = math.Log(value/d.lastValue) / weight * base
		d.lastValue = value
	}

	for i := 0; i < multiplier; i++ {
		d.chng.Update(chng)
	}

	if d.chng.Length() < d.window {
		return 0, false
	}

	stdev := types.Stdev(d.chng, d.window)
	return types.Mean(d.chng, d.window) - stdev*stdev*0.5, true
}

// zeroPoint assumes that the moving average is a SMA
func (d *weightedDriftState) zeroPoint() float64 {
	window := float64(d.window)
	stdev := types.Stdev(d.chng, d.window)
	chng := d.chng.Index(d.window - 1)
	ma := types.Mean(d.chng, d.window)
	return d.lastValue * math.Exp(window*(0.5*stdev*stdev)+chng-ma*window)
}

// WeightedDrift creates the volume weighted drift stream from the kline source,
// using the close price as the value and the absolute volume as the weight.
func WeightedDrift(source KLineSubscription, window int) *WeightedDriftStream {
	checkWindow(window)

	s := &WeightedDriftStream{
		Float64Series: types.NewFloat64Series(),
		state: weightedDriftState{
			window: window,
			chng:   types.NewQueue(window),
			weight: types.NewQueue(window),
		},
	}

	source.AddSubscriber(func(k types.KLine) {
		s.Update(k.Close.Float64(), k.Volume.Abs().Float64())
	})
	return s
}

// Update pushes a value with its weight into the stream.
func (s *WeightedDriftStream) Update(value, weight float64) {
	if drift, ok := s.state.update(value, weight); ok {
		s.PushAndEmit(drift)
		s.Truncate()
	}
}

// ZeroPoint returns the price that would bring the drift to the zero point
// (assuming that the moving average is a SMA).
func (s *WeightedDriftStream) ZeroPoint() float64 {
	return s.state.zeroPoint()
}

// TestUpdate returns the drift value as if the given value and weight were pushed,
// without changing the state of the stream.
// It returns 0 if the drift would not be ready yet.
func (s *WeightedDriftStream) TestUpdate(value, weight float64) float64 {
	st := s.state.clone()
	drift, _ := st.update(value, weight)
	return drift
}

func (s *WeightedDriftStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}
