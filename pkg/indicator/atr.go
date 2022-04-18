package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type ATR
type ATR struct {
	types.IntervalWindow
	Values               types.Float64Slice
	PercentageVolatility types.Float64Slice

	PriviousClose float64
	RMA           *RMA

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *ATR) Update(high, low, cloze float64) {
	if inc.Window <= 0 {
		panic("window must be greater than 0")
	}

	if len(inc.Values) == 0 {
		inc.RMA = &RMA{IntervalWindow: types.IntervalWindow{Window: inc.Window}}
	}

	if inc.PriviousClose == 0 {
		inc.PriviousClose = cloze
		return
	}

	// calculate true range
	trueRange := types.Float64Slice{
		high - low,
		math.Abs(high - inc.PriviousClose),
		math.Abs(low - inc.PriviousClose),
	}.Max()

	inc.PriviousClose = cloze

	// apply rolling moving average
	inc.RMA.Update(trueRange)
	atr := inc.RMA.Last()
	inc.Values.Push(atr)
	inc.PercentageVolatility.Push(atr / cloze)
}

func (inc *ATR) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *ATR) Index(i int) float64 {
	length := len(inc.Values)
	if length == 0 || length-i-1 < 0 {
		return 0
	}
	return inc.Values[length-i-1]
}

func (inc *ATR) Length() int {
	return len(inc.Values)
}

var _ types.Series = &ATR{}

func (inc *ATR) calculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}
		inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	}

	inc.EmitUpdate(inc.Last())
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}

func (inc *ATR) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *ATR) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
