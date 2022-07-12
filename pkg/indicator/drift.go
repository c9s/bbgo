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
	types.SeriesBase
	types.IntervalWindow
	chng      *types.Queue
	Values    types.Float64Slice
	SMA       *SMA
	LastValue float64

	UpdateCallbacks []func(value float64)
}

func (inc *Drift) Update(value float64) {
	if inc.chng == nil {
		inc.SeriesBase.Series = inc
		inc.SMA = &SMA{IntervalWindow: types.IntervalWindow{Interval: inc.Interval, Window: inc.Window}}
		inc.chng = types.NewQueue(inc.Window)
		inc.LastValue = value
		return
	}
	var chng float64
	if value == 0 {
		chng = 0
	} else {
		chng = math.Log(value / inc.LastValue)
		inc.LastValue = value
	}
	inc.SMA.Update(chng)
	inc.chng.Update(chng)
	if inc.chng.Length() >= inc.Window {
		stdev := types.Stdev(inc.chng, inc.Window)
		drift := inc.SMA.Last() - stdev*stdev*0.5
		inc.Values.Push(drift)
	}
}

func (inc *Drift) Clone() (out *Drift) {
	out = &Drift{
		IntervalWindow: inc.IntervalWindow,
		chng:           inc.chng.Clone(),
		Values:         inc.Values[:],
		SMA:            inc.SMA.Clone().(*SMA),
		LastValue:      inc.LastValue,
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *Drift) TestUpdate(value float64) *Drift {
	out := inc.Clone()
	out.Update(value)
	return out
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

var _ types.SeriesExtend = &Drift{}

func (inc *Drift) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *Drift) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.chng == nil {
		for _, k := range allKLines {
			inc.PushK(k)
			inc.EmitUpdate(inc.Last())
		}
	} else {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *Drift) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *Drift) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
