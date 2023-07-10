package fmaker

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type A2
type A2 struct {
	types.IntervalWindow

	// Values
	Values floats.Slice

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *A2) Last(int) float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *A2) CalculateAndUpdate(klines []types.KLine) {
	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	val, err := calculateA2(recentT, KLineLowPriceMapper, KLineHighPriceMapper, types.KLineClosePriceMapper)
	if err != nil {
		log.WithError(err).Error("can not calculate")
		return
	}
	inc.Values.Push(val)

	if len(inc.Values) > indicator.MaxNumOfVOL {
		inc.Values = inc.Values[indicator.MaxNumOfVOLTruncateSize-1:]
	}

	inc.EndTime = klines[end].GetEndTime().Time()

	inc.EmitUpdate(val)

}

func (inc *A2) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *A2) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

// (-1 * DELTA((((CLOSE - LOW) - (HIGH - CLOSE)) / (HIGH - LOW)), 1))
func calculateA2(klines []types.KLine, valLow KLineValueMapper, valHigh KLineValueMapper, valClose KLineValueMapper) (float64, error) {
	window := 2
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var lows floats.Slice
	var highs floats.Slice
	var closes floats.Slice

	for _, k := range klines {
		lows.Push(valLow(k))
		highs.Push(valHigh(k))
		closes.Push(valClose(k))
	}

	prev := ((closes.Last(1) - lows.Index(1)) - (highs.Index(1) - closes.Index(1))) / (highs.Index(1) - lows.Index(1))
	curr := ((closes.Last(0) - lows.Index(0)) - (highs.Index(0) - closes.Index(0))) / (highs.Index(0) - lows.Index(0))
	alpha := (curr - prev) * -1 // delta(1 interval)

	return alpha, nil
}

func KLineLowPriceMapper(k types.KLine) float64 {
	return k.Low.Float64()
}

func KLineHighPriceMapper(k types.KLine) float64 {
	return k.High.Float64()
}
