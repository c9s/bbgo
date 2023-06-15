package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// macd implements moving average convergence divergence indicator
//
// Moving Average Convergence Divergence (MACD)
// - https://www.investopedia.com/terms/m/macd.asp
// - https://school.stockcharts.com/doku.php?id=technical_indicators:macd-histogram
// The Moving Average Convergence Divergence (MACD) is a technical analysis indicator that is used to measure the relationship between
// two moving averages of a security's price. It is calculated by subtracting the longer-term moving average from the shorter-term moving
// average, and then plotting the resulting value on the price chart as a line. This line is known as the MACD line, and is typically
// used to identify potential buy or sell signals. The MACD is typically used in conjunction with a signal line, which is a moving average
// of the MACD line, to generate more accurate buy and sell signals.

type MACDConfig struct {
	types.IntervalWindow // 9

	// ShortPeriod is the short term period EMA, usually 12
	ShortPeriod int `json:"short"`
	// LongPeriod is the long term period EMA, usually 26
	LongPeriod int `json:"long"`
}

//go:generate callbackgen -type MACDLegacy
type MACDLegacy struct {
	MACDConfig

	Values                         floats.Slice `json:"-"`
	fastEWMA, slowEWMA, signalLine *EWMA
	Histogram                      floats.Slice `json:"-"`

	updateCallbacks []func(macd, signal, histogram float64)
	EndTime         time.Time
}

func (inc *MACDLegacy) Update(x float64) {
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
	fast := inc.fastEWMA.Last(0)
	slow := inc.slowEWMA.Last(0)
	macd := fast - slow
	inc.Values.Push(macd)

	// update signal line
	inc.signalLine.Update(macd)
	signal := inc.signalLine.Last(0)

	// update histogram
	histogram := macd - signal
	inc.Histogram.Push(histogram)

	inc.EmitUpdate(macd, signal, histogram)
}

func (inc *MACDLegacy) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *MACDLegacy) Length() int {
	return len(inc.Values)
}

func (inc *MACDLegacy) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *MACDLegacy) MACD() types.SeriesExtend {
	out := &MACDValues{MACDLegacy: inc}
	out.SeriesBase.Series = out
	return out
}

func (inc *MACDLegacy) Singals() types.SeriesExtend {
	return inc.signalLine
}

type MACDValues struct {
	types.SeriesBase
	*MACDLegacy
}

func (inc *MACDValues) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *MACDValues) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *MACDValues) Length() int {
	return len(inc.Values)
}
