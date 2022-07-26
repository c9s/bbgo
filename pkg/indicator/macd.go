package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

/*
macd implements moving average convergence divergence indicator

Moving Average Convergence Divergence (MACD)
- https://www.investopedia.com/terms/m/macd.asp
- https://school.stockcharts.com/doku.php?id=technical_indicators:macd-histogram
*/

//go:generate callbackgen -type MACD
type MACD struct {
	types.IntervalWindow     // 9
	ShortPeriod          int // 12
	LongPeriod           int // 26
	Values               types.Float64Slice
	FastEWMA             *EWMA
	SlowEWMA             *EWMA
	SignalLine           *EWMA
	Histogram            types.Float64Slice

	EndTime time.Time

	updateCallbacks []func(value float64)
}

func (inc *MACD) Update(x float64) {
	if len(inc.Values) == 0 {
		inc.FastEWMA = &EWMA{IntervalWindow: types.IntervalWindow{Window: inc.ShortPeriod}}
		inc.SlowEWMA = &EWMA{IntervalWindow: types.IntervalWindow{Window: inc.LongPeriod}}
		inc.SignalLine = &EWMA{IntervalWindow: types.IntervalWindow{Window: inc.Window}}
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

func (inc *MACD) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}

	return inc.Values[len(inc.Values)-1]
}

func (inc *MACD) Length() int {
	return len(inc.Values)
}

func (inc *MACD) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *MACD) MACD() types.SeriesExtend {
	out := &MACDValues{MACD: inc}
	out.SeriesBase.Series = out
	return out
}

func (inc *MACD) Singals() types.SeriesExtend {
	return inc.SignalLine
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
