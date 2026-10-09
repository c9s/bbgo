package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// Refer: https://www.investopedia.com/terms/a/adx.asp
// Refer: https://github.com/twopirllc/pandas-ta/blob/main/pandas_ta/trend/adx.py
//
// Average Directional Index (ADX)
//
// The Average Directional Index (ADX) is a technical analysis indicator used by some traders to determine
// the strength of a trend. The trend can be either up or down, and this is shown by two accompanying
// indicators, the Negative Directional Indicator (-DI) and the Positive Directional Indicator (+DI).
// Therefore, the ADX commonly includes three separate lines. These are used to help assess whether
// a trade should be taken long or short, or if a trade should be taken at all.

//go:generate callbackgen -type ADX
type ADX struct {
	types.SeriesBase
	types.IntervalWindow

	ADXSmoothing int
	dmi          *DMI

	EndTime         time.Time
	updateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &ADX{}

func (inc *ADX) Update(high, low, cloze float64) {
	if inc.dmi == nil {
		inc.SeriesBase.Series = inc
		smoothing := inc.ADXSmoothing
		if smoothing == 0 {
			smoothing = inc.Window
		}
		inc.dmi = &DMI{
			IntervalWindow: inc.IntervalWindow,
			ADXSmoothing:   smoothing,
		}
	}

	inc.dmi.Update(high, low, cloze)
}

func (inc *ADX) Last(i int) float64 {
	if inc.dmi == nil || inc.dmi.ADX == nil {
		return 0
	}
	return inc.dmi.ADX.Last(i)
}

func (inc *ADX) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *ADX) Length() int {
	if inc.dmi == nil || inc.dmi.ADX == nil {
		return 0
	}
	return inc.dmi.ADX.Length()
}

func (inc *ADX) PushK(k types.KLine) {
	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
}

func (inc *ADX) CalculateAndUpdate(allKLines []types.KLine) {
	if len(allKLines) == 0 {
		return
	}

	last := allKLines[len(allKLines)-1]

	if inc.dmi == nil || inc.dmi.ADX == nil {
		for _, k := range allKLines {
			inc.PushK(k)
			inc.EmitUpdate(inc.Last(0))
		}
	} else {
		inc.PushK(last)
		inc.EmitUpdate(inc.Last(0))
	}
	inc.EndTime = last.EndTime.Time()
}

func (inc *ADX) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *ADX) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
