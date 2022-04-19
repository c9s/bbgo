package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Zero Lag Exponential Moving Average
// Refer URL: https://en.wikipedia.org/wiki/Zero_lag_exponential_moving_average

//go:generate callbackgen -type ZLEMA
type ZLEMA struct {
	types.IntervalWindow

	data  *EWMA
	zlema *EWMA
	lag   int

	UpdateCallbacks []func(value float64)
}

func (inc *ZLEMA) Index(i int) float64 {
	return inc.zlema.Index(i)
}

func (inc *ZLEMA) Last() float64 {
	return inc.zlema.Last()
}

func (inc *ZLEMA) Length() int {
	return inc.zlema.Length()
}

func (inc *ZLEMA) Update(value float64) {
	if inc.lag == 0 || inc.zlema == nil {
		inc.data = &EWMA{IntervalWindow: types.IntervalWindow{inc.Interval, inc.Window}}
		inc.zlema = &EWMA{IntervalWindow: types.IntervalWindow{inc.Interval, inc.Window}}
		inc.lag = (inc.Window - 1) / 2
	}
	inc.data.Update(value)
	data := inc.data.Last()
	emaData := 2*data - inc.data.Index(inc.lag)
	inc.zlema.Update(emaData)
}

var _ types.Series = &ZLEMA{}

func (inc *ZLEMA) calculateAndUpdate(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.Update(k.Close.Float64())
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *ZLEMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *ZLEMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
