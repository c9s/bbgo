package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// Parabolic SAR(Stop and Reverse)
// Refer: https://www.investopedia.com/terms/p/parabolicindicator.asp
// The parabolic SAR indicator, developed by J. Wells Wilder, is used by traders to determine
// trend direction and potential reversals in price. The indicator uses a trailing stop and
// reverse method called "SAR," or stop and reverse, to identify suitable exit and entry points.
// Traders also refer to the indicator as to the parabolic stop and reverse, parabolic SAR, or PSAR.
//
// The parabolic SAR indicator appears on a chart as a series of dots, either above or below an asset's
// price, depending on the direction the price is moving. A dot is placed below the price when it is
// trending upward, and above the price when it is trending downward.

//go:generate callbackgen -type PSAR
type PSAR struct {
	types.SeriesBase
	types.IntervalWindow
	Input *types.Queue
	Value *types.Queue // Stop and Reverse
	AF    float64      // Acceleration Factor

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *PSAR) Last() float64 {
	if inc.Value == nil {
		return 0
	}
	return inc.Value.Last()
}

func (inc *PSAR) Length() int {
	if inc.Value == nil {
		return 0
	}
	return inc.Value.Length()
}

func (inc *PSAR) Update(value float64) {
	if inc.Input == nil {
		inc.SeriesBase.Series = inc
		inc.Input = types.NewQueue(inc.Window)
		inc.Value = types.NewQueue(inc.Window)
		inc.AF = 0.02
	}
	inc.Input.Update(value)
	if inc.Input.Length() == inc.Window {
		pprev := inc.Value.Index(1)
		ppsar := inc.Value.Last()
		if value > ppsar { // rising formula
			high := inc.Input.Highest(inc.Window)
			inc.Value.Update(ppsar + inc.AF*(high-ppsar))
			if high == value {
				inc.AF += 0.02
				if inc.AF > 0.2 {
					inc.AF = 0.2
				}
			}
			if pprev > ppsar { // reverse
				inc.AF = 0.02
			}
		} else { // falling formula
			low := inc.Input.Lowest(inc.Window)
			inc.Value.Update(ppsar - inc.AF*(ppsar-low))
			if low == value {
				inc.AF += 0.02
				if inc.AF > 0.2 {
					inc.AF = 0.2
				}
			}
			if pprev < ppsar { // reverse
				inc.AF = 0.02
			}
		}
	}
}

var _ types.SeriesExtend = &PSAR{}

func (inc *PSAR) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}
		inc.Update(k.Close.Float64())
	}

	inc.EmitUpdate(inc.Last())
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}

func (inc *PSAR) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *PSAR) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
