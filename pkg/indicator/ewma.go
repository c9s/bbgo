package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// These numbers should be aligned with bbgo MaxNumOfKLines and MaxNumOfKLinesTruncate
const MaxNumOfEWMA = 5_000
const MaxNumOfEWMATruncateSize = 100

//go:generate callbackgen -type EWMA
type EWMA struct {
	types.IntervalWindow
	types.SeriesBase

	Values  types.Float64Slice
	EndTime time.Time

	updateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &EWMA{}

func (inc *EWMA) Clone() *EWMA {
	out := &EWMA{
		IntervalWindow: inc.IntervalWindow,
		Values:         inc.Values[:],
		LastOpenTime:   inc.LastOpenTime,
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *EWMA) TestUpdate(value float64) *EWMA {
	out := inc.Clone()
	out.Update(value)
	return out
}

func (inc *EWMA) Update(value float64) {
	var multiplier = 2.0 / float64(1+inc.Window)

	if len(inc.Values) == 0 {
		inc.SeriesBase.Series = inc
		inc.Values.Push(value)
		return
	} else if len(inc.Values) > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
	}

	ema := (1-multiplier)*inc.Last() + multiplier*value
	inc.Values.Push(ema)
}

func (inc *EWMA) Last() float64 {
	if len(inc.Values) == 0 {
		return 0
	}

	return inc.Values[len(inc.Values)-1]
}

func (inc *EWMA) Index(i int) float64 {
	if i >= len(inc.Values) {
		return 0
	}

	return inc.Values[len(inc.Values)-1-i]
}

func (inc *EWMA) Length() int {
	return len(inc.Values)
}

func (inc *EWMA) CalculateAndUpdate(allKLines []types.KLine) {
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

func (inc *EWMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *EWMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func (inc *EWMA) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *EWMA) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last())
}

func (inc *EWMA) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
	inc.EmitUpdate(inc.Last())
}

func CalculateKLinesEMA(allKLines []types.KLine, priceF KLinePriceMapper, window int) float64 {
	var multiplier = 2.0 / (float64(window) + 1)
	return ewma(MapKLinePrice(allKLines, priceF), multiplier)
}

// see https://www.investopedia.com/ask/answers/122314/what-exponential-moving-average-ema-formula-and-how-ema-calculated.asp
func ewma(prices []float64, multiplier float64) float64 {
	var end = len(prices) - 1
	if end == 0 {
		return prices[0]
	}

	return prices[end]*multiplier + (1-multiplier)*ewma(prices[:end], multiplier)
}
