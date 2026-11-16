package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Rate of Change (ROC)
// - https://www.investopedia.com/terms/r/rateofchange.asp
//
// The Rate of Change (ROC) is a momentum-based technical indicator that measures
// the percentage change in price between the current price and the price n periods ago.
// Formula:
// ROC = ((Price - PrevPrice) / PrevPrice) * 100

//go:generate callbackgen -type ROC
type ROC struct {
	types.SeriesBase
	types.IntervalWindow

	Values floats.Slice
	Prices floats.Slice

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *ROC) Update(price float64) {
	if len(inc.Values) == 0 && len(inc.Prices) == 0 {
		inc.SeriesBase.Series = inc
	}
	inc.Prices.Push(price)

	if len(inc.Prices) <= inc.Window {
		return
	}

	prevPrice := inc.Prices[len(inc.Prices)-1-inc.Window]
	if prevPrice == 0 {
		inc.Values.Push(0.0)
	} else {
		roc := ((price - prevPrice) / prevPrice) * 100.0
		inc.Values.Push(roc)
	}
}

func (inc *ROC) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *ROC) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *ROC) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &ROC{}

func (inc *ROC) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
		return
	}

	inc.Update(k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
	if len(inc.Values) > 0 {
		inc.EmitUpdate(inc.Last(0))
	}
}

func (inc *ROC) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	if len(inc.Values) > 0 {
		inc.EmitUpdate(inc.Last(0))
	}
	if len(kLines) > 0 {
		inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
	}
}

func (inc *ROC) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *ROC) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
