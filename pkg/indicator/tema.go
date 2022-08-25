package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Triple Exponential Moving Average (TEMA)
// URL: https://investopedia.com/terms/t/triple-exponential-moving-average.asp

//go:generate callbackgen -type TEMA
type TEMA struct {
	types.SeriesBase
	types.IntervalWindow
	Values floats.Slice
	A1     *EWMA
	A2     *EWMA
	A3     *EWMA

	UpdateCallbacks []func(value float64)
}

func (inc *TEMA) Update(value float64) {
	if len(inc.Values) == 0 {
		inc.SeriesBase.Series = inc
		inc.A1 = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.A2 = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.A3 = &EWMA{IntervalWindow: inc.IntervalWindow}
	}
	inc.A1.Update(value)
	a1 := inc.A1.Last()
	inc.A2.Update(a1)
	a2 := inc.A2.Last()
	inc.A3.Update(a2)
	a3 := inc.A3.Last()
	inc.Values.Push(3*a1 - 3*a2 + a3)
}

func (inc *TEMA) Last() float64 {
	if len(inc.Values) > 0 {
		return inc.Values[len(inc.Values)-1]
	}
	return 0.0
}

func (inc *TEMA) Index(i int) float64 {
	if i >= len(inc.Values) {
		return 0
	}
	return inc.Values[len(inc.Values)-i-1]
}

func (inc *TEMA) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &TEMA{}

func (inc *TEMA) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *TEMA) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.A1 == nil {
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

func (inc *TEMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *TEMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
