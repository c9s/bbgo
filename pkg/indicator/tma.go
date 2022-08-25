package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Triangular Moving Average
// Refer URL: https://ja.wikipedia.org/wiki/移動平均
//go:generate callbackgen -type TMA
type TMA struct {
	types.SeriesBase
	types.IntervalWindow
	s1              *SMA
	s2              *SMA
	UpdateCallbacks []func(value float64)
}

func (inc *TMA) Update(value float64) {
	if inc.s1 == nil {
		inc.SeriesBase.Series = inc
		w := (inc.Window + 1) / 2
		inc.s1 = &SMA{IntervalWindow: types.IntervalWindow{Interval: inc.Interval, Window: w}}
		inc.s2 = &SMA{IntervalWindow: types.IntervalWindow{Interval: inc.Interval, Window: w}}
	}

	inc.s1.Update(value)
	inc.s2.Update(inc.s1.Last())
}

func (inc *TMA) Last() float64 {
	if inc.s2 == nil {
		return 0
	}
	return inc.s2.Last()
}

func (inc *TMA) Index(i int) float64 {
	if inc.s2 == nil {
		return 0
	}
	return inc.s2.Index(i)
}

func (inc *TMA) Length() int {
	if inc.s2 == nil {
		return 0
	}
	return inc.s2.Length()
}

var _ types.SeriesExtend = &TMA{}

func (inc *TMA) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *TMA) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.s1 == nil {
		for _, k := range allKLines {
			inc.PushK(k)
			inc.EmitUpdate(inc.Last())
		}
	} else {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *TMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *TMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
