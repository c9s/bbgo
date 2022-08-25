package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: https://easylanguagemastery.com/indicators/predictive-indicators/
// Refer: https://github.com/twopirllc/pandas-ta/blob/main/pandas_ta/overlap/ssf.py
// Ehler's Super Smoother Filter
//
// John F. Ehlers's solution to reduce lag and remove aliasing noise with his
// research in aerospace analog filter design. This indicator comes with two
// versions determined by the keyword poles. By default, it uses two poles but
// there is an option for three poles. Since SSF is a (Resursive) Digital Filter,
// the number of poles determine how many prior recursive SSF bars to include in
// the design of the filter. So two poles uses two prior SSF bars and three poles
// uses three prior SSF bars for their filter calculations.
//
//go:generate callbackgen -type SSF
type SSF struct {
	types.SeriesBase
	types.IntervalWindow
	Poles  int
	c1     float64
	c2     float64
	c3     float64
	c4     float64
	Values floats.Slice

	UpdateCallbacks []func(value float64)
}

func (inc *SSF) Update(value float64) {
	if inc.Poles == 3 {
		if inc.Values == nil {
			inc.SeriesBase.Series = inc
			x := math.Pi / float64(inc.Window)
			a0 := math.Exp(-x)
			b0 := 2. * a0 * math.Cos(math.Sqrt(3.)*x)
			c0 := a0 * a0

			inc.c4 = c0 * c0
			inc.c3 = -c0 * (1. + b0)
			inc.c2 = c0 + b0
			inc.c1 = 1. - inc.c2 - inc.c3 - inc.c4
			inc.Values = floats.Slice{}
		}

		result := inc.c1*value +
			inc.c2*inc.Values.Index(0) +
			inc.c3*inc.Values.Index(1) +
			inc.c4*inc.Values.Index(2)
		inc.Values.Push(result)
	} else { // poles == 2
		if inc.Values == nil {
			inc.SeriesBase.Series = inc
			x := math.Pi * math.Sqrt(2.) / float64(inc.Window)
			a0 := math.Exp(-x)
			inc.c3 = -a0 * a0
			inc.c2 = 2. * a0 * math.Cos(x)
			inc.c1 = 1. - inc.c2 - inc.c3
			inc.Values = floats.Slice{}
		}
		result := inc.c1*value +
			inc.c2*inc.Values.Index(0) +
			inc.c3*inc.Values.Index(1)
		inc.Values.Push(result)
	}
}

func (inc *SSF) Index(i int) float64 {
	if inc.Values == nil {
		return 0.0
	}
	return inc.Values.Index(i)
}

func (inc *SSF) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

func (inc *SSF) Last() float64 {
	if inc.Values == nil {
		return 0.0
	}
	return inc.Values.Last()
}

var _ types.SeriesExtend = &SSF{}

func (inc *SSF) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *SSF) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.Values != nil {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last())
		return
	}
	for _, k := range allKLines {
		inc.PushK(k)
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *SSF) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}
	inc.CalculateAndUpdate(window)
}

func (inc *SSF) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
