package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

type EWMA struct {
	types.IntervalWindow
	Values   Float64Slice
	EndTime  time.Time
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

	// see https://www.investopedia.com/ask/answers/122314/what-exponential-moving-average-ema-formula-and-how-ema-calculated.asp
	var multiplier = 2.0 / float64(inc.Window+1)

	if inc.EndTime != zeroTime && lastK.EndTime.Before(inc.EndTime) {
		return
	}

	var recentK = kLines[index-(inc.Window-1) : index+1]
	if len(inc.Values) > 0 {
		var previousEWMA = inc.Values[len(inc.Values)-1]
		var ewma = lastK.Close*multiplier + previousEWMA*(1-multiplier)
		inc.Values.Push(ewma)
	} else {
		// The first EWMA is actually SMA
		var sma = calculateSMA(recentK)
		inc.Values.Push(sma)
	}

	inc.EndTime = kLines[index].EndTime
}

type KLineWindowUpdater interface {
	OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow))
}

func (inc *EWMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	if inc.EndTime != zeroTime && inc.EndTime.Before(inc.EndTime) {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *EWMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
