package fmaker

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type S6
type S6 struct {
	types.IntervalWindow

	// Values
	Values floats.Slice

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *S6) Last(int) float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *S6) CalculateAndUpdate(klines []types.KLine) {
	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	val, err := calculateS6(recentT, types.KLineHighPriceMapper, types.KLineLowPriceMapper, types.KLineClosePriceMapper, types.KLineVolumeMapper)
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

func (inc *S6) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *S6) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateS6(klines []types.KLine, valHigh KLineValueMapper, valLow KLineValueMapper, valClose KLineValueMapper, valVolume KLineValueMapper) (float64, error) {
	window := 2
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var highs floats.Slice
	var lows floats.Slice
	var closes floats.Slice
	var volumes floats.Slice

	for _, k := range klines {
		highs.Push(valHigh(k))
		lows.Push(valLow(k))
		closes.Push(valClose(k))
		volumes.Push(valVolume(k))

	}

	H := highs.Last(0)
	L := lows.Last(0)
	C := closes.Last(0)
	V := volumes.Last(0)
	alpha := (H + L + C) / 3 * V

	return alpha, nil
}
