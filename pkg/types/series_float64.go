package types

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
)

type Float64Series struct {
	SeriesBase
	Float64Updater
	Slice floats.Slice
}

func NewFloat64Series(v ...float64) *Float64Series {
	s := &Float64Series{}
	s.Slice = v
	s.SeriesBase.Series = s.Slice
	return s
}

func (f *Float64Series) Last(i int) float64 {
	return f.Slice.Last(i)
}

func (f *Float64Series) Index(i int) float64 {
	return f.Last(i)
}

func (f *Float64Series) Length() int {
	return len(f.Slice)
}

// CrossOver returns true if the latest series values cross above x
func (f *Float64Series) CrossOver(curr, x float64) bool {
	if f.Last(1) < x && curr > x {
		return true
	}
	return false
}

// CrossDown returns true if the latest series values cross below x
func (f *Float64Series) CrossUnder(x float64) bool {
	if f.Last(1) > x && f.Last(0) < x {
		return true
	}
	return false
}

func (f *Float64Series) Push(x float64) {
	f.Slice.Push(x)
}

func (f *Float64Series) PushAndEmit(x float64) {
	f.Slice.Push(x)
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

	if source != nil {
		f.Subscribe(source, c)
	}
}

type Float64Calculator interface {
	Calculate(x float64) float64
	PushAndEmit(x float64)
}

type Float64Source interface {
	Series
	OnUpdate(f func(v float64))
}

type Float64Subscription interface {
	Series
	AddSubscriber(f func(v float64))
}

type Float64Truncator interface {
	Truncate()
}
