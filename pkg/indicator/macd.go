package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

type EMA struct {
	Window int
	Values Float64Slice
}

func (inc *EMA) Update(value float64) float64 {
	lambda := 2.0 / float64(1+inc.Window)

	length := len(inc.Values)
	var ema float64
	if length == 0 {
		ema = value
	} else {
		ema = (1-lambda)*inc.Values[length-1] + lambda*value
	}
	inc.Values.Push(ema)
	return ema
}

//go:generate callbackgen -type MACD
type MACD struct {
	types.IntervalWindow     // 9
	ShortPeriod          int // 12
	LongPeriod           int // 26
	Values               Float64Slice
	FastEMA              EMA
	SlowEMA              EMA
	SignalLine           EMA
	Histogram            Float64Slice

	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *MACD) calculateMACD(kLines []types.KLine, priceF KLinePriceMapper) float64 {
	for _, kline := range kLines {
		inc.update(kline, priceF)
	}
	return inc.Values.LastValue()
}

func (inc *MACD) update(kLine types.KLine, priceF KLinePriceMapper) {
	if len(inc.Values) == 0 {
		inc.FastEMA = EMA{Window: inc.ShortPeriod}
		inc.SlowEMA = EMA{Window: inc.LongPeriod}
		inc.SignalLine = EMA{Window: inc.Window}
	}

	price := priceF(kLine)

	// update fast and slow ema
	fastEMA := inc.FastEMA.Update(price)
	slowEMA := inc.SlowEMA.Update(price)

	// update macd
	macd := fastEMA - slowEMA
	inc.Values.Push(macd)

	// update signal line
	signalValue := inc.SignalLine.Update(macd)

	// update histogram
	inc.Histogram.Push(macd - signalValue)
}

func (inc *MACD) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
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
		inc.EmitUpdate(inc.Values.LastValue())
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
