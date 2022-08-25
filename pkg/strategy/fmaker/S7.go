package fmaker

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type S7
type S7 struct {
	types.IntervalWindow

	// Values
	Values floats.Slice

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *S7) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *S7) CalculateAndUpdate(klines []types.KLine) {
	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	val, err := calculateS7(recentT, indicator.KLineOpenPriceMapper, indicator.KLineClosePriceMapper)
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

func (inc *S7) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *S7) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateS7(klines []types.KLine, valOpen KLineValueMapper, valClose KLineValueMapper) (float64, error) {
	window := 2
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var opens floats.Slice
	var closes floats.Slice

	for _, k := range klines {
		opens.Push(valOpen(k))
		closes.Push(valClose(k))

	}

	O := opens.Last()
	C := closes.Last()
	alpha := -(1 - O/C)

	return alpha, nil
}
