package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Running Moving Average
//go:generate callbackgen -type RMA
type RMA struct {
	types.IntervalWindow
	Values  types.Float64Slice
	Sources types.Float64Slice

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *RMA) Update(x float64) {
	inc.Sources.Push(x)

	if len(inc.Sources) < inc.Window {
		inc.Values.Push(0)
		return
	}

	if len(inc.Sources) == inc.Window {
		inc.Values.Push(inc.Sources.Mean())
		return
	}

	lambda := 1 / float64(inc.Window)
	rma := (1-lambda)*inc.Values.Last() + lambda*x
	inc.Values.Push(rma)
}

func (inc *RMA) Last() float64 {
	return inc.Values.Last()
}

func (inc *RMA) Index(i int) float64 {
	length := len(inc.Values)
	if length == 0 || length-i-1 < 0 {
		return 0
	}
	return inc.Values[length-i-1]
}

func (inc *RMA) Length() int {
	return len(inc.Values)
}

var _ types.Series = &RMA{}

func (inc *RMA) calculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}
		inc.Update(k.Close.Float64())
	}

	inc.EmitUpdate(inc.Last())
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}
func (inc *RMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *RMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
