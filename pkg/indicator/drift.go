package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// Refer: https://tradingview.com/script/aDymGrFx-Drift-Study-Inspired-by-Monte-Carlo-Simulations-with-BM-KL/
// Brownian Motion's drift factor
// could be used in Monte Carlo Simulations
//go:generate callbackgen -type Drift
type Drift struct {
	types.IntervalWindow
	chng   *types.Queue
	Values types.Float64Slice
	SMA    *SMA
	LastValue float64

	UpdateCallbacks []func(value float64)
}

func (inc *Drift) Update(value float64) {
	if inc.chng == nil {
		inc.SMA = &SMA{IntervalWindow: types.IntervalWindow{inc.Interval, inc.Window}}
		inc.chng = types.NewQueue(inc.Window)
	}
	chng := math.Log(value / inc.LastValue)
	inc.LastValue = value
	inc.SMA.Update(chng)
	inc.chng.Update(chng)
	stdev := types.Stdev(inc.chng, inc.Window)
	drift := inc.SMA.Last() - stdev * stdev * 0.5
	inc.Values.Push(drift)
}

func (inc *Drift) Index(i int) float64 {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Index(i)
}

func (inc *Drift) Last() float64 {
	if inc.Values.Length() == 0 {
		return 0
	}
	return inc.Values.Last()
}

func (inc *Drift) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

var _ types.Series = &Drift{}

func (inc *Drift) calculateAndUpdate(allKLines []types.KLine) {
	if inc.chng == nil {
		for _, k := range allKLines {
			inc.Update(k.Close.Float64())
			inc.EmitUpdate(inc.Last())
		}
	} else {
		inc.Update(allKLines[len(allKLines)-1].Close.Float64())
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *Drift) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *Drift) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
