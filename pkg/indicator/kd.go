package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

const DPeriod int = 3

/*
kd implements stochastic oscillator indicator

Stochastic Oscillator
- https://www.investopedia.com/terms/s/stochasticoscillator.asp
*/
//go:generate callbackgen -type KD
type KD struct {
	types.IntervalWindow
	K Float64Slice
	D Float64Slice

	KLineWindow types.KLineWindow

	EndTime         time.Time
	UpdateCallbacks []func(k float64, d float64)
}

func (inc *KD) update(kLine types.KLine) {
	inc.KLineWindow.Add(kLine)
	inc.KLineWindow.Truncate(inc.Window)

	lowest := inc.KLineWindow.GetLow()
	highest := inc.KLineWindow.GetHigh()

	k := 100.0 * (kLine.Close - lowest) / (highest - lowest)
	inc.K.Push(k)

	d := inc.K.Tail(DPeriod).Mean()
	inc.D.Push(d)
}

func (inc *KD) LastK() float64 {
	if len(inc.K) == 0 {
		return 0.0
	}
	return inc.K[len(inc.K)-1]
}

func (inc *KD) LastD() float64 {
	if len(inc.K) == 0 {
		return 0.0
	}
	return inc.D[len(inc.D)-1]
}

func (inc *KD) calculateAndUpdate(kLines []types.KLine) {
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

func (inc *KD) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *KD) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
