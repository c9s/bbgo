package factorzoo

import (
	"time"

	"gonum.org/v1/gonum/stat"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// price mean reversion
// assume that the quotient of SMA over close price will dynamically revert into one.
// so this fraction value is our alpha, PMR

//go:generate callbackgen -type PMR
type PMR struct {
	types.IntervalWindow
	types.SeriesBase

	Values floats.Slice
	SMA    *indicator.SMA
	EndTime time.Time

	updateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &PMR{}

func (inc *PMR) Update(price float64) {
	if inc.SeriesBase.Series == nil {
		inc.SeriesBase.Series = inc
		inc.SMA = &indicator.SMA{IntervalWindow: inc.IntervalWindow}
	}
	inc.SMA.Update(price)
	if inc.SMA.Length() >= inc.Window {
		reversion := inc.SMA.Last() / price
		inc.Values.Push(reversion)
	}
}

func (inc *PMR) Last() float64 {
	if len(inc.Values) == 0 {
		return 0
	}

	return inc.Values[len(inc.Values)-1]
}

func (inc *PMR) Index(i int) float64 {
	if i >= len(inc.Values) {
		return 0
	}

	return inc.Values[len(inc.Values)-1-i]
}

func (inc *PMR) Length() int {
	return len(inc.Values)
}

func (inc *PMR) CalculateAndUpdate(allKLines []types.KLine) {
	if len(inc.Values) == 0 {
		for _, k := range allKLines {
			inc.PushK(k)
		}
		inc.EmitUpdate(inc.Last())
	} else {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *PMR) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *PMR) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func (inc *PMR) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(indicator.KLineClosePriceMapper(k))
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last())
}

func CalculateKLinesPMR(allKLines []types.KLine, window int) float64 {
	return pmr(indicator.MapKLinePrice(allKLines, indicator.KLineClosePriceMapper), window)
}

func pmr(prices []float64, window int) float64 {
	var end = len(prices) - 1
	if end == 0 {
		return prices[0]
	}

	reversion := -stat.Mean(prices[end-window:end], nil) / prices[end]
	return reversion
}
