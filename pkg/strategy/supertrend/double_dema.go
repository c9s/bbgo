package supertrend

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type DoubleDema struct {
	Interval types.Interval `json:"interval"`

	// FastDEMAWindow DEMA window for checking breakout
	FastDEMAWindow int `json:"fastDEMAWindow"`
	// SlowDEMAWindow DEMA window for checking breakout
	SlowDEMAWindow int `json:"slowDEMAWindow"`
	fastDEMA       *indicator.DEMA
	slowDEMA       *indicator.DEMA
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

// preloadDema preloads DEMA indicators
func (dd *DoubleDema) preloadDema(kLineStore *bbgo.MarketDataStore) {
	if klines, ok := kLineStore.KLinesOfInterval(dd.fastDEMA.Interval); ok {
		for i := 0; i < len(*klines); i++ {
			dd.fastDEMA.Update((*klines)[i].GetClose().Float64())
		}
	}
	if klines, ok := kLineStore.KLinesOfInterval(dd.slowDEMA.Interval); ok {
		for i := 0; i < len(*klines); i++ {
			dd.slowDEMA.Update((*klines)[i].GetClose().Float64())
		}
	}
}

// newDoubleDema initializes double DEMA indicators
func newDoubleDema(kLineStore *bbgo.MarketDataStore, interval types.Interval, fastDEMAWindow int, slowDEMAWindow int) *DoubleDema {
	dd := DoubleDema{Interval: interval, FastDEMAWindow: fastDEMAWindow, SlowDEMAWindow: slowDEMAWindow}

	// DEMA
	if dd.FastDEMAWindow == 0 {
		dd.FastDEMAWindow = 144
	}
	dd.fastDEMA = &indicator.DEMA{IntervalWindow: types.IntervalWindow{Interval: dd.Interval, Window: dd.FastDEMAWindow}}
	dd.fastDEMA.Bind(kLineStore)

	if dd.SlowDEMAWindow == 0 {
		dd.SlowDEMAWindow = 169
	}
	dd.slowDEMA = &indicator.DEMA{IntervalWindow: types.IntervalWindow{Interval: dd.Interval, Window: dd.SlowDEMAWindow}}
	dd.slowDEMA.Bind(kLineStore)

	dd.preloadDema(kLineStore)

	return &dd
}
