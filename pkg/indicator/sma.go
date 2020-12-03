package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

type Float64Slice []float64

func (s *Float64Slice) Push(v float64) {
	*s = append(*s, v)
}

var zeroTime time.Time


//go:generate callbackgen -type SMA
type SMA struct {
	types.IntervalWindow
	Values  Float64Slice
	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *SMA) Last() float64 {
	return inc.Values[len(inc.Values)-1]
}

func (inc *SMA) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		return
	}

	var index = len(kLines) - 1
	var kline = kLines[index]

	if inc.EndTime != zeroTime && kline.EndTime.Before(inc.EndTime) {
		return
	}

	var recentK = kLines[index-(inc.Window-1) : index+1]
	var sma = calculateSMA(recentK)
	inc.Values.Push(sma)
	inc.EndTime = kLines[index].EndTime

	inc.EmitUpdate(sma)
}

func (inc *SMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *SMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateSMA(kLines []types.KLine) float64 {
	length := len(kLines)

	if length == 0 {
		return 0.0
	}

	sum := 0.0
	for _, k := range kLines {
		sum += k.Close
	}

	avg := sum / float64(length)
	return avg
}
