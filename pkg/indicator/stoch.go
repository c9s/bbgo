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

	KLineWindow types.KLineWindow

	EndTime         time.Time
	UpdateCallbacks []func(k float64, d float64)
}

func (inc *STOCH) update(kLine types.KLine) {
	inc.KLineWindow.Add(kLine)
	inc.KLineWindow.Truncate(inc.Window)

	lowest := inc.KLineWindow.GetLow().Float64()
	highest := inc.KLineWindow.GetHigh().Float64()
	clos := kLine.Close.Float64()

	k := 100.0 * (clos - lowest) / (highest - lowest)
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

	for _, k := range kLines {
		if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
			continue
		}
		inc.update(k)
	}

	inc.EmitUpdate(inc.LastK(), inc.LastD())
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
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

func (inc *STOCH) GetD() *DSeries {
	return &DSeries{inc}
}

func (inc *STOCH) GetK() *KSeries {
	return &KSeries{inc}
}

type DSeries struct {
	*STOCH
}

func (inc *DSeries) Last() float64 {
	return inc.LastD()
}

func (inc *DSeries) Length() int {
	return len(inc.D)
}

func (inc *DSeries) Index(i int) float64 {
	length := len(inc.D)
	if length == 0 || length - i - 1 < 0 {
		return 0
	}
	return inc.D[length - i - 1]
}

var _ types.Series = &DSeries{}

type KSeries struct {
	*STOCH
}

func (inc *KSeries) Last() float64 {
	return inc.LastK()
}

func (inc *KSeries) Index(i int) float64 {
	length := len(inc.K)
	if length == 0 || length - i - 1 < 0 {
		return 0
	}
	return inc.K[length - i - 1]
}

func (inc *KSeries) Length() int {
	return len(inc.K)
}

var _ types.Series = &KSeries{}
