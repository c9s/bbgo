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

	Lows   floats.Slice
	Values floats.Slice
	KLines []types.KLine

	EndTime time.Time

	updateCallbacks []func(value float64)
}

func (inc *PivotLow) Length() int {
	return inc.Values.Length()
}

func (inc *PivotLow) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}

	return inc.Values.Last()
}

func (inc *PivotLow) update(value float64, k types.KLine) {
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
		inc.KLines = append(inc.KLines, k)
	}
}

func (inc *PivotLow) PushK(k types.KLine) {
	if k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.update(k.Low.Float64(), k)
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last())
}

func calculatePivotHigh(highs floats.Slice, left, right int) (float64, bool) {
	return floats.CalculatePivot(highs, left, right, func(a, pivot float64) bool {
		return a < pivot
	})
}

func calculatePivotLow(lows floats.Slice, left, right int) (float64, bool) {
	return floats.CalculatePivot(lows, left, right, func(a, pivot float64) bool {
		return a > pivot
	})
}
