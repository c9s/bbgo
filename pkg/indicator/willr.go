package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Williams %R (Williams Percent Range)
// - https://www.investopedia.com/terms/w/williamsr.asp
//
// The Williams %R is a momentum indicator that measures overbought and oversold levels,
// comparing a security's close to the high-low range over a specific period (typically 14 periods).
// Formula:
// %R = (Highest High - Close) / (Highest High - Lowest Low) * -100
// Values range from 0 to -100.
// Readings between 0 and -20 are considered overbought, while readings between -80 and -100 are considered oversold.

//go:generate callbackgen -type WILLR
type WILLR struct {
	types.SeriesBase
	types.IntervalWindow

	Values     floats.Slice
	HighValues floats.Slice
	LowValues  floats.Slice

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *WILLR) Update(high, low, cloze float64) {
	if len(inc.Values) == 0 {
		inc.SeriesBase.Series = inc
	}
	inc.HighValues.Push(high)
	inc.LowValues.Push(low)

	lowest := inc.LowValues.Tail(inc.Window).Min()
	highest := inc.HighValues.Tail(inc.Window).Max()

	if highest == lowest {
		inc.Values.Push(-50.0)
	} else {
		val := -100.0 * (highest - cloze) / (highest - lowest)
		inc.Values.Push(val)
	}
}

func (inc *WILLR) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *WILLR) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *WILLR) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &WILLR{}

func (inc *WILLR) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
		return
	}

	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last(0))
}

func (inc *WILLR) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	if len(inc.Values) > 0 {
		inc.EmitUpdate(inc.Last(0))
	}
	if len(kLines) > 0 {
		inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
	}
}

func (inc *WILLR) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *WILLR) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
