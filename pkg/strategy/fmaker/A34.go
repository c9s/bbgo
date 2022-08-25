package fmaker

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type A34
type A34 struct {
	types.IntervalWindow

	// Values
	Values floats.Slice

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *A34) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *A34) CalculateAndUpdate(klines []types.KLine) {
	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	val, err := calculateA34(recentT, indicator.KLineClosePriceMapper)
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

func (inc *A34) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *A34) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateA34(klines []types.KLine, valClose KLineValueMapper) (float64, error) {
	window := 12
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var closes floats.Slice

	for _, k := range klines {
		closes.Push(valClose(k))
	}

	c := closes.Last()

	sumC := 0.
	for i := 1; i <= 12; i++ {
		sumC += closes.Index(len(closes) - i)
	}

	meanC := sumC / 12

	alpha := meanC / c

	return alpha, nil
}
