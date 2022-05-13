package pivotshort

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

var zeroTime time.Time

type KLineValueMapper func(k types.KLine) float64

//go:generate callbackgen -type Pivot
type Pivot struct {
	types.IntervalWindow

	// Values
	Lows  types.Float64Slice // higher low
	Highs types.Float64Slice // lower high

	EndTime time.Time

	UpdateCallbacks []func(valueLow, valueHigh float64)
}

func (inc *Pivot) LastLow() float64 {
	if len(inc.Lows) == 0 {
		return 0.0
	}
	return inc.Lows[len(inc.Lows)-1]
}

func (inc *Pivot) LastHigh() float64 {
	if len(inc.Highs) == 0 {
		return 0.0
	}
	return inc.Highs[len(inc.Highs)-1]
}

func (inc *Pivot) calculateAndUpdate(klines []types.KLine) {
	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	l, h, err := calculatePivot(recentT, inc.Window, KLineLowPriceMapper, KLineHighPriceMapper)
	if err != nil {
		log.WithError(err).Error("can not calculate pivots")
		return
	}
	inc.Lows.Push(l)
	inc.Highs.Push(h)

	if len(inc.Lows) > indicator.MaxNumOfVOL {
		inc.Lows = inc.Lows[indicator.MaxNumOfVOLTruncateSize-1:]
	}
	if len(inc.Highs) > indicator.MaxNumOfVOL {
		inc.Highs = inc.Highs[indicator.MaxNumOfVOLTruncateSize-1:]
	}

	inc.EndTime = klines[end].GetEndTime().Time()

	inc.EmitUpdate(l, h)

}

func (inc *Pivot) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *Pivot) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculatePivot(klines []types.KLine, window int, valLow KLineValueMapper, valHigh KLineValueMapper) (float64, float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0., 0., fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
	}
	var lows types.Float64Slice
	var highs types.Float64Slice
	for _, k := range klines {
		lows.Push(valLow(k))
		highs.Push(valHigh(k))
	}

	pl := 0.
	if lows.Min() == lows.Index(int(window/2.)-1) {
		pl = lows.Min()
	}
	ph := 0.
	if highs.Max() == highs.Index(int(window/2.)-1) {
		ph = highs.Max()
	}

	return pl, ph, nil
}

func KLineLowPriceMapper(k types.KLine) float64 {
	return k.Low.Float64()
}

func KLineHighPriceMapper(k types.KLine) float64 {
	return k.High.Float64()
}
