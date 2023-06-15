package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Triple Exponential Moving Average (TEMA)
// URL: https://investopedia.com/terms/t/triple-exponential-moving-average.asp
//
// The Triple Exponential Moving Average (TEMA) is a technical analysis indicator that is used to smooth price data and reduce the lag
// associated with traditional moving averages. It is calculated by taking the exponentially weighted moving average of the input data,
// and then taking the exponentially weighted moving average of that result, and then taking the exponentially weighted moving average of
// that result. This triple-smoothing process helps to eliminate much of the noise in the original data and provides a more accurate
// representation of the underlying trend. The TEMA line is then plotted on the price chart, which can be used to make predictions about
// future price movements. The TEMA is typically more responsive to changes in the underlying data than a simple moving average, but may be
// less reliable in trending markets.

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
	a1 := inc.A1.Last(0)
	inc.A2.Update(a1)
	a2 := inc.A2.Last(0)
	inc.A3.Update(a2)
	a3 := inc.A3.Last(0)
	inc.Values.Push(3*a1 - 3*a2 + a3)
}

func (inc *TEMA) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *TEMA) Index(i int) float64 {
	return inc.Last(i)
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
			inc.EmitUpdate(inc.Last(0))
		}
	} else {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last(0))
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
