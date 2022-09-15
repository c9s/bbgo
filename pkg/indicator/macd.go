package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

/*
macd implements moving average convergence divergence indicator

Moving Average Convergence Divergence (MACD)
- https://www.investopedia.com/terms/m/macd.asp
- https://school.stockcharts.com/doku.php?id=technical_indicators:macd-histogram
*/
type MACDConfig struct {
	types.IntervalWindow // 9

	// ShortPeriod is the short term period EMA, usually 12
	ShortPeriod int `json:"short"`
	// LongPeriod is the long term period EMA, usually 26
	LongPeriod int `json:"long"`
}

//go:generate callbackgen -type MACD
type MACD struct {
	MACDConfig

	Values                         floats.Slice `json:"-"`
	fastEWMA, slowEWMA, signalLine *EWMA
	Histogram                      floats.Slice `json:"-"`

	EndTime time.Time

	updateCallbacks []func(macd, signal, histogram float64)
}

func (inc *MACD) Update(x float64) {
	if len(inc.Values) == 0 {
		// apply default values
		inc.fastEWMA = &EWMA{IntervalWindow: types.IntervalWindow{Window: inc.ShortPeriod}}
		inc.slowEWMA = &EWMA{IntervalWindow: types.IntervalWindow{Window: inc.LongPeriod}}
		inc.signalLine = &EWMA{IntervalWindow: types.IntervalWindow{Window: inc.Window}}
		if inc.ShortPeriod == 0 {
			inc.ShortPeriod = 12
		}

		if inc.LongPeriod == 0 {
			inc.LongPeriod = 26
		}
	}

	// update fast and slow ema
	inc.fastEWMA.Update(x)
	inc.slowEWMA.Update(x)

	// update MACD value, it's also the signal line
	fast := inc.fastEWMA.Last()
	slow := inc.slowEWMA.Last()
	macd := fast - slow
	inc.Values.Push(macd)

	// update signal line
	inc.signalLine.Update(macd)
	signal := inc.signalLine.Last()

	// update histogram
	histogram := macd - signal
	inc.Histogram.Push(histogram)

	inc.EmitUpdate(macd, signal, histogram)
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
	return inc.signalLine
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
