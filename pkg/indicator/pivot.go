package indicator

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)


//go:generate callbackgen -type Pivot
type Pivot struct {
	types.IntervalWindow

	// Values
	Lows  floats.Slice // higher low
	Highs floats.Slice // lower high

	EndTime time.Time

	updateCallbacks []func(valueLow, valueHigh float64)
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

func (inc *Pivot) CalculateAndUpdate(klines []types.KLine) {
	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	// skip old data
	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	recentT := klines[end-(inc.Window-1) : end+1]

	l, h, err := calculatePivot(recentT, inc.Window, KLineLowPriceMapper, KLineHighPriceMapper)
	if err != nil {
		log.WithError(err).Error("can not calculate pivots")
		return
	}

	if l > 0.0 {
		inc.Lows.Push(l)
	}
	if h > 0.0 {
		inc.Highs.Push(h)
	}

	if len(inc.Lows) > MaxNumOfVOL {
		inc.Lows = inc.Lows[MaxNumOfVOLTruncateSize-1:]
	}
	if len(inc.Highs) > MaxNumOfVOL {
		inc.Highs = inc.Highs[MaxNumOfVOLTruncateSize-1:]
	}

	inc.EndTime = klines[end].GetEndTime().Time()

	inc.EmitUpdate(l, h)

}

func (inc *Pivot) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *Pivot) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculatePivot(klines []types.KLine, window int, valLow KLineValueMapper, valHigh KLineValueMapper) (float64, float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0., 0., fmt.Errorf("insufficient elements for calculating with window = %d", window)
	}

	var lows floats.Slice
	var highs floats.Slice
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
