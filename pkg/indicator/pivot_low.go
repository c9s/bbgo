package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type PivotLow
type PivotLow struct {
	types.SeriesBase

	types.IntervalWindow

	RightWindow int `json:"rightWindow"`

	Lows    types.Float64Slice
	Values  types.Float64Slice
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
	inc.EmitUpdate(inc.Last())
}

func calculatePivotLow(lows types.Float64Slice, left, right int) (float64, bool) {
	length := len(lows)

	if right == 0 {
		right = left
	}

	if length == 0 || length < left+right+1 {
		return 0.0, false
	}

	end := length - 1
	index := end - right
	val := lows[index]

	for i := index - left; i <= index+right; i++ {
		if i == index {
			continue
		}

		// return if we found lower value
		if lows[i] < val {
			return 0.0, false
		}
	}

	return val, true
}
