package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

const DPeriod int = 3

/*
stoch implements stochastic oscillator indicator

Stochastic Oscillator
- https://www.investopedia.com/terms/s/stochasticoscillator.asp
*/
//go:generate callbackgen -type STOCH
type STOCH struct {
	types.IntervalWindow
	K types.Float64Slice
	D types.Float64Slice

	HighValues types.Float64Slice
	LowValues  types.Float64Slice

	EndTime         time.Time
	UpdateCallbacks []func(k float64, d float64)
}

func (inc *STOCH) Update(high, low, cloze float64) {
	inc.HighValues.Push(high)
	inc.LowValues.Push(low)

	lowest := inc.LowValues.Tail(inc.Window).Min()
	highest := inc.HighValues.Tail(inc.Window).Max()

	if highest == lowest {
		inc.K.Push(50.0)
	} else {
		k := 100.0 * (cloze - lowest) / (highest - lowest)
		inc.K.Push(k)
	}

	d := inc.K.Tail(DPeriod).Mean()
	inc.D.Push(d)
}

func (inc *STOCH) LastK() float64 {
	if len(inc.K) == 0 {
		return 0.0
	}
	return inc.K[len(inc.K)-1]
}

func (inc *STOCH) LastD() float64 {
	if len(inc.K) == 0 {
		return 0.0
	}
	return inc.D[len(inc.D)-1]
}

func (inc *STOCH) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
		return
	}

	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
}

func (inc *STOCH) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *STOCH) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {

		inc.PushK(k)
	}
	inc.EmitUpdate(inc.LastK(), inc.LastD())
}

func (inc *STOCH) CalculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window || len(kLines) < DPeriod {
		return
	}

	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	inc.EmitUpdate(inc.LastK(), inc.LastD())
}

func (inc *STOCH) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *STOCH) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func (inc *STOCH) GetD() types.Series {
	return &inc.D
}

func (inc *STOCH) GetK() types.Series {
	return &inc.K
}
