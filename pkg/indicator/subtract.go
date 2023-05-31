package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
)

// SubtractStream subscribes 2 upstream data, and then subtract these 2 values
type SubtractStream struct {
	Float64Series

	a, b floats.Slice
	i    int
}

// Subtract creates the SubtractStream object
// subtract := Subtract(longEWMA, shortEWMA)
func Subtract(a, b Float64Source) *SubtractStream {
	s := &SubtractStream{
		Float64Series: NewFloat64Series(),
	}

	a.OnUpdate(func(v float64) {
		s.a.Push(v)
		s.calculate()
	})
	b.OnUpdate(func(v float64) {
		s.b.Push(v)
		s.calculate()
	})
	return s
}

func (s *SubtractStream) calculate() {
	if s.a.Length() != s.b.Length() {
		return
	}

	if s.a.Length() > s.slice.Length() {
		var numNewElems = s.a.Length() - s.slice.Length()
		var tailA = s.a.Tail(numNewElems)
		var tailB = s.b.Tail(numNewElems)
		var tailC = tailA.Sub(tailB)
		for _, f := range tailC {
			s.slice.Push(f)
			s.EmitUpdate(f)
		}
	}
}
