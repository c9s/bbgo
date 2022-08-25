package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// ATRP is the average true range percentage
// See also https://www.fidelity.com/learning-center/trading-investing/technical-analysis/technical-indicator-guide/atrp
//
// Calculation:
//
//     ATRP = (Average True Range / Close) * 100
//
//go:generate callbackgen -type ATRP
type ATRP struct {
	types.SeriesBase
	types.IntervalWindow
	PercentageVolatility floats.Slice

	PreviousClose float64
	RMA           *RMA

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *ATRP) Update(high, low, cloze float64) {
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

	// Note: this is the difference from ATR
	trueRange = trueRange / inc.PreviousClose * 100.0

	inc.PreviousClose = cloze

	// apply rolling moving average
	inc.RMA.Update(trueRange)
	atr := inc.RMA.Last()
	inc.PercentageVolatility.Push(atr / cloze)
}

func (inc *ATRP) Last() float64 {
	if inc.RMA == nil {
		return 0
	}
	return inc.RMA.Last()
}

func (inc *ATRP) Index(i int) float64 {
	if inc.RMA == nil {
		return 0
	}
	return inc.RMA.Index(i)
}

func (inc *ATRP) Length() int {
	if inc.RMA == nil {
		return 0
	}
	return inc.RMA.Length()
}

var _ types.SeriesExtend = &ATRP{}

func (inc *ATRP) PushK(k types.KLine) {
	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
}

func (inc *ATRP) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	inc.EmitUpdate(inc.Last())
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}

func (inc *ATRP) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *ATRP) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
