package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type PivotHigh
type PivotHigh struct {
	types.SeriesBase

	types.IntervalWindow

	Highs   floats.Slice
	Values  floats.Slice
	EndTime time.Time

	updateCallbacks []func(value float64)
}

func (inc *PivotHigh) Length() int {
	return inc.Values.Length()
}

func (inc *PivotHigh) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *PivotHigh) Update(value float64) {
	if len(inc.Highs) == 0 {
		inc.SeriesBase.Series = inc
	}

	inc.Highs.Push(value)

	if len(inc.Highs) < inc.Window {
		return
	}

	if inc.RightWindow == nil {
		inc.RightWindow = &inc.Window
	}

	high, ok := calculatePivotHigh(inc.Highs, inc.Window, *inc.RightWindow)
	if !ok {
		return
	}

	if high > 0.0 {
		inc.Values.Push(high)
	}
}

func (inc *PivotHigh) PushK(k types.KLine) {
	if k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.High.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last(0))
}
