package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

/*
macd implements moving average convergence divergence indicator

Moving Average Convergence Divergence (MACD)
- https://www.investopedia.com/terms/m/macd.asp
*/

//go:generate callbackgen -type MACD
type MACD struct {
	types.IntervalWindow     // 9
	ShortPeriod          int // 12
	LongPeriod           int // 26
	Values               types.Float64Slice
	FastEWMA             EWMA
	SlowEWMA             EWMA
	SignalLine           EWMA
	Histogram            types.Float64Slice

	EndTime time.Time

	updateCallbacks []func(value float64)
}

func (inc *MACD) Update(x float64) {
	if len(inc.Values) == 0 {
		inc.FastEWMA = EWMA{IntervalWindow: types.IntervalWindow{Window: inc.ShortPeriod}}
		inc.SlowEWMA = EWMA{IntervalWindow: types.IntervalWindow{Window: inc.LongPeriod}}
		inc.SignalLine = EWMA{IntervalWindow: types.IntervalWindow{Window: inc.Window}}
	}

	// update fast and slow ema
	inc.FastEWMA.Update(x)
	inc.SlowEWMA.Update(x)

	// update macd
	macd := inc.FastEWMA.Last() - inc.SlowEWMA.Last()
	inc.Values.Push(macd)

	// update signal line
	inc.SignalLine.Update(macd)

	// update histogram
	inc.Histogram.Push(macd - inc.SignalLine.Last())
}

// Deprecated -- this function is not used ??? ask @narumi
func (inc *MACD) calculateMACD(kLines []types.KLine, priceF KLinePriceMapper) float64 {
	for _, k := range kLines {
		inc.PushK(k)
	}

	return inc.Values[len(inc.Values)-1]
}

func (inc *MACD) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *MACD) CalculateAndUpdate(kLines []types.KLine) {
	if len(kLines) == 0 {
		return
	}

	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	inc.EmitUpdate(inc.Values[len(inc.Values)-1])
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}

func (inc *MACD) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *MACD) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

type MACDValues struct {
	types.SeriesBase
	*MACD
}

func (inc *MACDValues) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *MACDValues) Index(i int) float64 {
	length := len(inc.Values)
	if length == 0 || length-1-i < 0 {
		return 0.0
	}
	return inc.Values[length-1+i]
}

func (inc *MACDValues) Length() int {
	return len(inc.Values)
}

func (inc *MACD) MACD() types.SeriesExtend {
	out := &MACDValues{MACD: inc}
	out.SeriesBase.Series = out
	return out
}

func (inc *MACD) Singals() types.SeriesExtend {
	return &inc.SignalLine
}
