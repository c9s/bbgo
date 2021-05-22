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
	K Float64Slice
	D Float64Slice

	KLineWindow types.KLineWindow

	EndTime         time.Time
	UpdateCallbacks []func(k float64, d float64)
}

func (inc *STOCH) update(kLine types.KLine) {
	inc.KLineWindow.Add(kLine)
	inc.KLineWindow.Truncate(inc.Window)

	lowest := inc.KLineWindow.GetLow()
	highest := inc.KLineWindow.GetHigh()

	k := 100.0 * (kLine.Close - lowest) / (highest - lowest)
	inc.K.Push(k)

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

func (inc *STOCH) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window || len(kLines) < DPeriod {
		return
	}

	for i, k := range kLines {
		if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
			continue
		}

		inc.update(k)
		inc.EmitUpdate(inc.LastK(), inc.LastD())
		inc.EndTime = kLines[i].EndTime
	}
}

func (inc *STOCH) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *STOCH) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
