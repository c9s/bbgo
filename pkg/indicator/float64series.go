package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

type Float64Series struct {
	types.SeriesBase
	Float64Updater
	slice floats.Slice
}

func NewFloat64Series(v ...float64) Float64Series {
	s := Float64Series{}
	s.slice = v
	s.SeriesBase.Series = s.slice
	return s
}

func (f *Float64Series) Last(i int) float64 {
	return f.slice.Last(i)
}

func (f *Float64Series) Index(i int) float64 {
	return f.Last(i)
}

func (f *Float64Series) Length() int {
	return len(f.slice)
}

func (f *Float64Series) Slice() floats.Slice {
	return f.slice
}

func (f *Float64Series) PushAndEmit(x float64) {
	f.slice.Push(x)
	f.EmitUpdate(x)
}

func (f *Float64Series) Subscribe(source Float64Source, c func(x float64)) {
	if sub, ok := source.(Float64Subscription); ok {
		sub.AddSubscriber(c)
	} else {
		source.OnUpdate(c)
	}
}

// Bind binds the source event to the target (Float64Calculator)
// A Float64Calculator should be able to calculate the float64 result from a single float64 argument input
func (f *Float64Series) Bind(source Float64Source, target Float64Calculator) {
	var c func(x float64)

	// optimize the truncation check
	trc, canTruncate := target.(Float64Truncator)
	if canTruncate {
		c = func(x float64) {
			y := target.Calculate(x)
			target.PushAndEmit(y)
			trc.Truncate()
		}
	} else {
		c = func(x float64) {
			y := target.Calculate(x)
			target.PushAndEmit(y)
		}
	}

	f.Subscribe(source, c)
}
