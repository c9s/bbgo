package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Zero Lag Exponential Moving Average
// Refer URL: https://en.wikipedia.org/wiki/Zero_lag_exponential_moving_average
//
// The Zero Lag Exponential Moving Average (ZLEMA) is a technical analysis indicator that is used to smooth price data and reduce the
// lag associated with traditional moving averages. It is calculated by taking the exponentially weighted moving average of the input
// data, and then applying a digital filter to the resulting average to eliminate any remaining lag. This filtered average is then
// plotted on the price chart as a line, which can be used to make predictions about future price movements. The ZLEMA is typically more
// responsive to changes in the underlying data than a simple moving average, but may be less reliable in trending markets.

//go:generate callbackgen -type ZLEMA
type ZLEMA struct {
	types.SeriesBase
	types.IntervalWindow

	data  floats.Slice
	zlema *EWMA
	lag   int

	updateCallbacks []func(value float64)
}

func (inc *ZLEMA) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *ZLEMA) Last(i int) float64 {
	if inc.zlema == nil {
		return 0
	}
	return inc.zlema.Last(i)
}

func (inc *ZLEMA) Length() int {
	if inc.zlema == nil {
		return 0
	}
	return inc.zlema.Length()
}

func (inc *ZLEMA) Update(value float64) {
	if inc.lag == 0 || inc.zlema == nil {
		inc.SeriesBase.Series = inc
		inc.zlema = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.lag = int((float64(inc.Window)-1.)/2. + 0.5)
	}
	inc.data.Push(value)
	if len(inc.data) > MaxNumOfEWMA {
		inc.data = inc.data[MaxNumOfEWMATruncateSize-1:]
	}
	if inc.lag >= inc.data.Length() {
		return
	}
	emaData := 2.*value - inc.data[len(inc.data)-1-inc.lag]
	inc.zlema.Update(emaData)
}

var _ types.SeriesExtend = &ZLEMA{}

func (inc *ZLEMA) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *ZLEMA) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.zlema == nil {
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

func (inc *ZLEMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *ZLEMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
