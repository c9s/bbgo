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
// The Average True Range Percentage (ATRP) is a technical analysis indicator that measures the volatility of a security's price. It is
// calculated by dividing the Average True Range (ATR) of the security by its closing price, and then multiplying the result by 100 to convert
// it to a percentage. The ATR is a measure of the range of a security's price, taking into account gaps between trading periods and any limit
// moves (sharp price movements that are allowed under certain exchange rules). The ATR is typically smoothed using a moving average to make it
// more responsive to changes in the underlying price data. The ATRP is a useful indicator for traders because it provides a way to compare the
// volatility of different securities, regardless of their individual prices. It can also be used to identify potential entry and exit points
// for trades based on changes in the security's volatility.
//
// Calculation:
//
//	ATRP = (Average True Range / Close) * 100
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
	atr := inc.RMA.Last(0)
	inc.PercentageVolatility.Push(atr / cloze)
}

func (inc *ATRP) Last(i int) float64 {
	if inc.RMA == nil {
		return 0
	}
	return inc.RMA.Last(i)
}

func (inc *ATRP) Index(i int) float64 {
	return inc.Last(i)
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

	inc.EmitUpdate(inc.Last(0))
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
