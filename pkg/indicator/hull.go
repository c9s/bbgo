package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Hull Moving Average
// Refer URL: https://fidelity.com/learning-center/trading-investing/technical-analysis/technical-indicator-guide/hull-moving-average
//go:generate callbackgen -type HULL
type HULL struct {
	types.SeriesBase
	types.IntervalWindow
	ma1    *EWMA
	ma2    *EWMA
	result *EWMA

	updateCallbacks []func(value float64)
}

func (inc *HULL) Update(value float64) {
	if inc.result == nil {
		inc.SeriesBase.Series = inc
		inc.ma1 = &EWMA{IntervalWindow: types.IntervalWindow{inc.Interval, inc.Window / 2}}
		inc.ma2 = &EWMA{IntervalWindow: types.IntervalWindow{inc.Interval, inc.Window}}
		inc.result = &EWMA{IntervalWindow: types.IntervalWindow{inc.Interval, int(math.Sqrt(float64(inc.Window)))}}
	}
	inc.ma1.Update(value)
	inc.ma2.Update(value)
	inc.result.Update(2*inc.ma1.Last() - inc.ma2.Last())
}

func (inc *HULL) Last() float64 {
	if inc.result == nil {
		return 0
	}
	return inc.result.Last()
}

func (inc *HULL) Index(i int) float64 {
	if inc.result == nil {
		return 0
	}
	return inc.result.Index(i)
}

func (inc *HULL) Length() int {
	if inc.result == nil {
		return 0
	}
	return inc.result.Length()
}

var _ types.SeriesExtend = &HULL{}

// TODO: should we just ignore the possible overlapping?
func (inc *HULL) calculateAndUpdate(allKLines []types.KLine) {
	doable := false
	if inc.ma1 == nil || inc.ma1.Length() == 0 {
		doable = true
	}
	for _, k := range allKLines {
		if !doable && k.StartTime.After(inc.ma1.LastOpenTime) {
			doable = true
		}
		if doable {
			inc.Update(k.Close.Float64())
			inc.EmitUpdate(inc.Last())
		}
	}
}

func (inc *HULL) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *HULL) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
