package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Welles Wilder's Moving Average
// Refer URL: http://fxcorporate.com/help/MS/NOTFIFO/i_WMA.html
// TODO: Cannot see any difference between RMA and this

const MaxNumOfWWMA = 5_000
const MaxNumOfWWMATruncateSize = 100

//go:generate callbackgen -type WWMA
type WWMA struct {
	types.SeriesBase
	types.IntervalWindow
	Values       floats.Slice
	LastOpenTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *WWMA) Update(value float64) {
	if len(inc.Values) == 0 {
		inc.SeriesBase.Series = inc
		inc.Values.Push(value)
		return
	} else if len(inc.Values) > MaxNumOfWWMA {
		inc.Values = inc.Values[MaxNumOfWWMATruncateSize-1:]
	}

	last := inc.Last()
	wma := last + (value-last)/float64(inc.Window)
	inc.Values.Push(wma)
}

func (inc *WWMA) Last() float64 {
	if len(inc.Values) == 0 {
		return 0
	}

	return inc.Values[len(inc.Values)-1]
}

func (inc *WWMA) Index(i int) float64 {
	if i >= len(inc.Values) {
		return 0
	}

	return inc.Values[len(inc.Values)-1-i]
}

func (inc *WWMA) Length() int {
	return len(inc.Values)
}

func (inc *WWMA) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *WWMA) CalculateAndUpdate(allKLines []types.KLine) {
	if len(allKLines) < inc.Window {
		// we can't calculate
		return
	}

	doable := false
	for _, k := range allKLines {
		if !doable && k.StartTime.After(inc.LastOpenTime) {
			doable = true
		}
		if doable {
			inc.PushK(k)
			inc.LastOpenTime = k.StartTime.Time()
			inc.EmitUpdate(inc.Last())
		}
	}
}

func (inc *WWMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *WWMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

var _ types.SeriesExtend = &WWMA{}
