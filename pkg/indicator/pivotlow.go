package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type PivotLow
type PivotLow struct {
	types.SeriesBase

	types.IntervalWindow

	Lows    floats.Slice
	Values  floats.Slice
	EndTime time.Time

	updateCallbacks []func(value float64)
}

func (inc *PivotLow) Length() int {
	return inc.Values.Length()
}

func (inc *PivotLow) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *PivotLow) Update(value float64) {
	if len(inc.Lows) == 0 {
		inc.SeriesBase.Series = inc
	}

	inc.Lows.Push(value)

	if len(inc.Lows) < inc.Window {
		return
	}

	low, ok := calculatePivotLow(inc.Lows, inc.Window, inc.RightWindow)
	if !ok {
		return
	}

	if low > 0.0 {
		inc.Values.Push(low)
	}
}

func (inc *PivotLow) PushK(k types.KLine) {
	if k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.Low.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last(0))
}

func calculatePivotHigh(highs floats.Slice, left, right int) (float64, bool) {
	return floats.FindPivot(highs, left, right, func(a, pivot float64) bool {
		return a < pivot
	})
}

func calculatePivotLow(lows floats.Slice, left, right int) (float64, bool) {
	return floats.FindPivot(lows, left, right, func(a, pivot float64) bool {
		return a > pivot
	})
}
