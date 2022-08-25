package fmaker

import (
	"fmt"
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type A3
type A3 struct {
	types.IntervalWindow

	// Values
	Values floats.Slice

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *A3) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *A3) CalculateAndUpdate(klines []types.KLine) {
	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	val, err := calculateA3(recentT, KLineLowPriceMapper, KLineHighPriceMapper, indicator.KLineClosePriceMapper)
	if err != nil {
		log.WithError(err).Error("can not calculate pivots")
		return
	}
	inc.Values.Push(val)

	if len(inc.Values) > indicator.MaxNumOfVOL {
		inc.Values = inc.Values[indicator.MaxNumOfVOLTruncateSize-1:]
	}

	inc.EndTime = klines[end].GetEndTime().Time()

	inc.EmitUpdate(val)

}

func (inc *A3) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *A3) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

// SUM((CLOSE = DELAY(CLOSE, 1)?0:CLOSE-(CLOSE>DELAY(CLOSE, 1)?MIN(LOW, DELAY(CLOSE, 1)):MAX(HIGH, DELAY(CLOSE, 1)))), 6)
func calculateA3(klines []types.KLine, valLow KLineValueMapper, valHigh KLineValueMapper, valClose KLineValueMapper) (float64, error) {
	window := 6 + 2
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

	a := 0.
	sumA := 0.
	for i := 1; i <= 6; i++ {
		if closes.Index(len(closes)-i) == closes.Index(len(closes)-i-1) {
			a = 0.
		} else {
			if closes.Index(len(closes)-i) > closes.Index(1) {
				a = closes.Index(len(closes)-i) - math.Min(lows.Index(len(lows)-i), closes.Index(len(closes)-i-1))
			} else {
				a = closes.Index(len(closes)-i) - math.Max(highs.Index(len(highs)-i), closes.Index(len(closes)-i-1))
			}
		}
		sumA += a
	}

	alpha := sumA // sum(a, 6 interval)

	return alpha, nil
}
