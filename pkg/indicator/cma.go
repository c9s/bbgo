package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Cumulative Moving Average, Cumulative Average
// Refer: https://en.wikipedia.org/wiki/Moving_average
//go:generate callbackgen -type CA
type CA struct {
	types.SeriesBase
	Interval types.Interval
	Values   floats.Slice
	length   float64
	UpdateCallbacks []func(value float64)
}

func (inc *CA) Update(x float64) {
	if len(inc.Values) == 0 {
		inc.SeriesBase.Series = inc
	}
	newVal := (inc.Values.Last()*inc.length + x) / (inc.length + 1.)
	inc.length += 1
	inc.Values.Push(newVal)
	if len(inc.Values) > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
		inc.length = float64(len(inc.Values))
	}
}

func (inc *CA) Last() float64 {
	if len(inc.Values) == 0 {
		return 0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *CA) Index(i int) float64 {
	if i >= len(inc.Values) {
		return 0
	}
	return inc.Values[len(inc.Values)-1-i]
}

func (inc *CA) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &CA{}

func (inc *CA) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *CA) CalculateAndUpdate(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *CA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *CA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
