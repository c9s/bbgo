package fmaker

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type S5
type S5 struct {
	types.IntervalWindow

	// Values
	Values floats.Slice

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *S5) Last(int) float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *S5) CalculateAndUpdate(klines []types.KLine) {
	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	val, err := calculateS5(recentT, types.KLineVolumeMapper)
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

func (inc *S5) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *S5) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateS5(klines []types.KLine, valVolume KLineValueMapper) (float64, error) {
	window := 10
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var volumes floats.Slice

	for _, k := range klines {
		volumes.Push(valVolume(k))
	}

	v := volumes.Last(0)

	sumV := 0.
	for i := 1; i <= 10; i++ {
		sumV += volumes.Index(len(volumes) - i)
	}

	meanV := sumV / 10

	alpha := -v / meanV

	return alpha, nil
}
