package supertrend

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type DoubleDema struct {
	Interval types.Interval `json:"interval"`

	// FastDEMAWindow DEMA window for checking breakout
	FastDEMAWindow int `json:"fastDEMAWindow"`
	// SlowDEMAWindow DEMA window for checking breakout
	SlowDEMAWindow int `json:"slowDEMAWindow"`
	fastDEMA       *indicatorv2.DEMAStream
	slowDEMA       *indicatorv2.DEMAStream
}

// getDemaSignal get current DEMA signal
func (dd *DoubleDema) getDemaSignal(openPrice float64, closePrice float64) types.Direction {
	var demaSignal types.Direction = types.DirectionNone

	if closePrice > dd.fastDEMA.Last(0) && closePrice > dd.slowDEMA.Last(0) && !(openPrice > dd.fastDEMA.Last(0) && openPrice > dd.slowDEMA.Last(0)) {
		demaSignal = types.DirectionUp
	} else if closePrice < dd.fastDEMA.Last(0) && closePrice < dd.slowDEMA.Last(0) && !(openPrice < dd.fastDEMA.Last(0) && openPrice < dd.slowDEMA.Last(0)) {
		demaSignal = types.DirectionDown
	}

	return demaSignal
}

// newDoubleDema initializes double DEMA indicators on the close prices of the given interval,
// the kline history of the market data store is loaded by the indicator set.
func newDoubleDema(indicators *bbgo.IndicatorSet, interval types.Interval, fastDEMAWindow int, slowDEMAWindow int) *DoubleDema {
	dd := DoubleDema{Interval: interval, FastDEMAWindow: fastDEMAWindow, SlowDEMAWindow: slowDEMAWindow}

	// DEMA
	if dd.FastDEMAWindow == 0 {
		dd.FastDEMAWindow = 144
	}
	dd.fastDEMA = indicators.DEMA(types.IntervalWindow{Interval: dd.Interval, Window: dd.FastDEMAWindow})

	if dd.SlowDEMAWindow == 0 {
		dd.SlowDEMAWindow = 169
	}
	dd.slowDEMA = indicators.DEMA(types.IntervalWindow{Interval: dd.Interval, Window: dd.SlowDEMAWindow})

	return &dd
}
