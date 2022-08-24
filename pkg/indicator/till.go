package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

const defaultVolumeFactor = 0.7

// Refer: Tillson T3 Moving Average
// Refer URL: https://tradingpedia.com/forex-trading-indicator/t3-moving-average-indicator/
//go:generate callbackgen -type TILL
type TILL struct {
	types.SeriesBase
	types.IntervalWindow
	VolumeFactor float64
	e1           *EWMA
	e2           *EWMA
	e3           *EWMA
	e4           *EWMA
	e5           *EWMA
	e6           *EWMA
	c1           float64
	c2           float64
	c3           float64
	c4           float64

	updateCallbacks []func(value float64)
}

func (inc *TILL) Update(value float64) {
	if inc.e1 == nil || inc.e1.Length() == 0 {
		if inc.VolumeFactor == 0 {
			inc.VolumeFactor = defaultVolumeFactor
		}
		inc.SeriesBase.Series = inc
		inc.e1 = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.e2 = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.e3 = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.e4 = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.e5 = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.e6 = &EWMA{IntervalWindow: inc.IntervalWindow}
		square := inc.VolumeFactor * inc.VolumeFactor
		cube := inc.VolumeFactor * square
		inc.c1 = -cube
		inc.c2 = 3.*square + 3.*cube
		inc.c3 = -6.*square - 3*inc.VolumeFactor - 3*cube
		inc.c4 = 1. + 3.*inc.VolumeFactor + cube + 3.*square
	}

	inc.e1.Update(value)
	inc.e2.Update(inc.e1.Last())
	inc.e3.Update(inc.e2.Last())
	inc.e4.Update(inc.e3.Last())
	inc.e5.Update(inc.e4.Last())
	inc.e6.Update(inc.e5.Last())
}

func (inc *TILL) Last() float64 {
	if inc.e1 == nil || inc.e1.Length() == 0 {
		return 0
	}
	e3 := inc.e3.Last()
	e4 := inc.e4.Last()
	e5 := inc.e5.Last()
	e6 := inc.e6.Last()
	return inc.c1*e6 + inc.c2*e5 + inc.c3*e4 + inc.c4*e3
}

func (inc *TILL) Index(i int) float64 {
	if inc.e1 == nil || inc.e1.Length() <= i {
		return 0
	}
	e3 := inc.e3.Index(i)
	e4 := inc.e4.Index(i)
	e5 := inc.e5.Index(i)
	e6 := inc.e6.Index(i)
	return inc.c1*e6 + inc.c2*e5 + inc.c3*e4 + inc.c4*e3
}

func (inc *TILL) Length() int {
	if inc.e1 == nil {
		return 0
	}
	return inc.e1.Length()
}

var _ types.Series = &TILL{}

func (inc *TILL) PushK(k types.KLine) {
	if inc.e1 != nil && inc.e1.EndTime != zeroTime && k.EndTime.Before(inc.e1.EndTime) {
		return
	}

	inc.Update(k.Close.Float64())
	inc.EmitUpdate(inc.Last())
}

func (inc *TILL) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
}

func (inc *TILL) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *TILL) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.e1 == nil {
		for _, k := range allKLines {
			inc.PushK(k)
		}
	} else {
		end := len(allKLines)
		last := allKLines[end-1]
		inc.PushK(last)
	}

}

func (inc *TILL) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *TILL) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
