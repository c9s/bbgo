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
	Values               Float64Slice
	FastEWMA             EWMA
	SlowEWMA             EWMA
	SignalLine           EWMA
	Histogram            Float64Slice

	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *MACD) calculateMACD(kLines []types.KLine, priceF KLinePriceMapper) float64 {
	for _, kline := range kLines {
		inc.update(kline, priceF)
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *MACD) update(kLine types.KLine, priceF KLinePriceMapper) {
	if len(inc.Values) == 0 {
		inc.FastEWMA = EWMA{IntervalWindow: types.IntervalWindow{Window: inc.ShortPeriod}}
		inc.SlowEWMA = EWMA{IntervalWindow: types.IntervalWindow{Window: inc.LongPeriod}}
		inc.SignalLine = EWMA{IntervalWindow: types.IntervalWindow{Window: inc.Window}}
	}

	price := priceF(kLine)

	// update fast and slow ema
	inc.FastEWMA.Update(price)
	inc.SlowEWMA.Update(price)

	// update macd
	macd := inc.FastEWMA.Last() - inc.SlowEWMA.Last()
	inc.Values.Push(macd)

	// update signal line
	inc.SignalLine.Update(macd)

	// update histogram
	inc.Histogram.Push(macd - inc.SignalLine.Last())
}

func (inc *MACD) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) == 0 {
		return
	}

	var priceF = KLineClosePriceMapper

	var index = len(kLines) - 1
	var kline = kLines[index]
	if inc.EndTime != zeroTime && kline.EndTime.Before(inc.EndTime) {
		return
	}

	for i, kLine := range kLines {
		inc.update(kLine, priceF)
		inc.EmitUpdate(inc.Values[len(inc.Values)-1])
		inc.EndTime = kLines[i].EndTime
	}

}

func (inc *MACD) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *MACD) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
