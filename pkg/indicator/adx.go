package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// Average Directional Index (ADX)
//
// The Average Directional Index (ADX) is a technical analysis indicator used to measure the overall strength of a trend.
// Developed by J. Welles Wilder, it is derived from the smoothed moving average of the Directional Movement Index (DMI).
//
// Values above 20 or 25 typically signify a strong trend, while values below indicate a weak or absent trend.
//
//go:generate callbackgen -type ADX
type ADX struct {
	types.SeriesBase
	types.IntervalWindow

	ADXSmoothing int
	dmi          *DMI

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *ADX) Update(high, low, cloze float64) {
	if inc.Window <= 0 {
		panic("window must be greater than 0")
	}

	if inc.dmi == nil {
		inc.SeriesBase.Series = inc
		smoothing := inc.ADXSmoothing
		if smoothing <= 0 {
			smoothing = inc.Window
		}
		inc.dmi = &DMI{
			IntervalWindow: types.IntervalWindow{
				Interval: inc.Interval,
				Window:   inc.Window,
			},
			ADXSmoothing: smoothing,
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

var _ types.SeriesExtend = &ADX{}

func (inc *ADX) PushK(k types.KLine) {
	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
}

func (inc *ADX) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	inc.EmitUpdate(inc.Last(0))
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
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
