package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type ATR
type ATR struct {
	types.SeriesBase
	types.IntervalWindow
	PercentageVolatility types.Float64Slice

	PreviousClose float64
	RMA           *RMA

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &ATR{}

func (inc *ATR) Update(high, low, cloze float64) {
	if inc.Window <= 0 {
		panic("window must be greater than 0")
	}

	if inc.RMA == nil {
		inc.SeriesBase.Series = inc
		inc.RMA = &RMA{
			IntervalWindow: types.IntervalWindow{Window: inc.Window},
			Adjust:         true,
		}
		inc.PreviousClose = cloze
		return
	}

	// calculate true range
	trueRange := high - low
	hc := math.Abs(high - inc.PreviousClose)
	lc := math.Abs(low - inc.PreviousClose)
	if trueRange < hc {
		trueRange = hc
	}
	if trueRange < lc {
		trueRange = lc
	}

	inc.PreviousClose = cloze

	// apply rolling moving average
	inc.RMA.Update(trueRange)
	atr := inc.RMA.Last()
	inc.PercentageVolatility.Push(atr / cloze)
}

func (inc *ATR) Last() float64 {
	if inc.RMA == nil {
		return 0
	}
	return inc.RMA.Last()
}

func (inc *ATR) Index(i int) float64 {
	if inc.RMA == nil {
		return 0
	}
	return inc.RMA.Index(i)
}

func (inc *ATR) Length() int {
	if inc.RMA == nil {
		return 0
	}

	return inc.RMA.Length()
}

func (inc *ATR) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
		return
	}

	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last())
}

func (inc *ATR) LoadK(allKlines []types.KLine) {
	for _, k := range allKlines {
		inc.PushK(k)
	}
}

func (inc *ATR) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *ATR) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		inc.PushK(k)
	}
}

func (inc *ATR) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *ATR) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
