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
	TrueRanges           types.Float64Slice
	PercentageVolatility types.Float64Slice
	PriviousClose        float64

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *ATR) Update(kLine types.KLine) {
	if inc.Window <= 0 {
		panic("window must be greater than 0")
	}

	cloze := kLine.Close.Float64()
	high := kLine.High.Float64()
	low := kLine.Low.Float64()

	if inc.PriviousClose == 0 {
		inc.PriviousClose = kLine.Close.Float64()
		return
	}

	// calculate true range
	trueRange := types.Float64Slice{
		high - low,
		math.Abs(high - inc.PriviousClose),
		math.Abs(low - inc.PriviousClose),
	}.Max()
	inc.TrueRanges.Push(trueRange)

	inc.PriviousClose = cloze

	// apply rolling moving average
	if len(inc.TrueRanges) < inc.Window {
		return
	}

	if len(inc.TrueRanges) == inc.Window {
		atr := inc.TrueRanges.Mean()
		inc.Values.Push(atr)
		inc.PercentageVolatility.Push(atr / cloze)
		return
	}

	lambda := 1 / float64(inc.Window)
	atr := inc.Values.Last()*(1-lambda) + inc.TrueRanges.Last()*lambda
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
		inc.Update(k)
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
