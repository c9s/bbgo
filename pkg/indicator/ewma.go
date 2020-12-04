package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type EWMA
type EWMA struct {
	types.IntervalWindow
	Values  Float64Slice
	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *EWMA) Last() float64 {
	return inc.Values[len(inc.Values)-1]
}

func (inc *EWMA) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		// we can't calculate
		return
	}

	var index = len(kLines) - 1
	var lastK = kLines[index]
	if inc.EndTime != zeroTime && lastK.EndTime.Before(inc.EndTime) {
		return
	}

	var recentK = kLines[index-(inc.Window-1) : index+1]
	var multiplier = 2.0 / float64(inc.Window+1)
	var val = calculateEWMA(recentK, multiplier)
	// val = calculateSMA(recentK)
	inc.Values.Push(val)
	inc.EndTime = lastK.EndTime
	inc.EmitUpdate(val)
}

// see https://www.investopedia.com/ask/answers/122314/what-exponential-moving-average-ema-formula-and-how-ema-calculated.asp
func calculateEWMA(kLines []types.KLine, multiplier float64) float64 {
	var end = len(kLines) - 1
	if end == 0 {
		return kLines[0].Close
	}

	return kLines[end].Close*multiplier + (1-multiplier)*calculateEWMA(kLines[:end-1], multiplier)
}

type KLineWindowUpdater interface {
	OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow))
}

func (inc *EWMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *EWMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
