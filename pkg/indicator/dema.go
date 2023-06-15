package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Double Exponential Moving Average
// Refer URL: https://investopedia.com/terms/d/double-exponential-moving-average.asp
//
// The Double Exponential Moving Average (DEMA) is a technical analysis indicator that is used to smooth price data and reduce the lag
// associated with traditional moving averages. It is calculated by taking the exponentially weighted moving average of the input data,
// and then taking the exponentially weighted moving average of that result. This double-smoothing process helps to eliminate much of the noise
// in the original data and provides a more accurate representation of the underlying trend. The DEMA line is then plotted on the price chart,
// which can be used to make predictions about future price movements. The DEMA is typically more responsive to changes in the underlying data
// than a simple moving average, but may be less reliable in trending markets.

//go:generate callbackgen -type DEMA
type DEMA struct {
	types.IntervalWindow
	types.SeriesBase
	Values floats.Slice
	a1     *EWMA
	a2     *EWMA

	UpdateCallbacks []func(value float64)
}

func (inc *DEMA) Clone() *DEMA {
	out := &DEMA{
		IntervalWindow: inc.IntervalWindow,
		Values:         inc.Values[:],
		a1:             inc.a1.Clone(),
		a2:             inc.a2.Clone(),
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *DEMA) TestUpdate(value float64) *DEMA {
	out := inc.Clone()
	out.Update(value)
	return out
}

func (inc *DEMA) Update(value float64) {
	if len(inc.Values) == 0 {
		inc.SeriesBase.Series = inc
		inc.a1 = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.a2 = &EWMA{IntervalWindow: inc.IntervalWindow}
	}

	inc.a1.Update(value)
	inc.a2.Update(inc.a1.Last(0))
	inc.Values.Push(2*inc.a1.Last(0) - inc.a2.Last(0))
	if len(inc.Values) > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
	}
}

func (inc *DEMA) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *DEMA) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *DEMA) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &DEMA{}

func (inc *DEMA) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *DEMA) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.a1 == nil {
		for _, k := range allKLines {
			inc.PushK(k)
			inc.EmitUpdate(inc.Last(0))
		}
	} else {
		// last k
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last(0))
	}
}

func (inc *DEMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *DEMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
