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

var _ types.SeriesExtend = &HULL{}

func (inc *HULL) Update(value float64) {
	if inc.result == nil {
		inc.SeriesBase.Series = inc
		inc.ma1 = &EWMA{IntervalWindow: types.IntervalWindow{Interval: inc.Interval, Window: inc.Window / 2}}
		inc.ma2 = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.result = &EWMA{IntervalWindow: types.IntervalWindow{Interval: inc.Interval, Window: int(math.Sqrt(float64(inc.Window)))}}
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

func (inc *HULL) PushK(k types.KLine) {
	if inc.ma1 != nil && inc.ma1.Length() > 0 && k.EndTime.Before(inc.ma1.EndTime) {
		return
	}

	inc.Update(k.Close.Float64())
	inc.EmitUpdate(inc.Last())
}
