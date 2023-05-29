package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// SubtractStream subscribes 2 upstream data, and then subtract these 2 values
type SubtractStream struct {
	Float64Updater
	types.SeriesBase

	a, b, c floats.Slice
	i       int
}

// Subtract creates the SubtractStream object
// subtract := Subtract(longEWMA, shortEWMA)
func Subtract(a, b Float64Source) *SubtractStream {
	s := &SubtractStream{}
	s.SeriesBase.Series = s.c

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

	if s.a.Length() > s.c.Length() {
		var numNewElems = s.a.Length() - s.c.Length()
		var tailA = s.a.Tail(numNewElems)
		var tailB = s.b.Tail(numNewElems)
		var tailC = tailA.Sub(tailB)
		for _, f := range tailC {
			s.c.Push(f)
			s.EmitUpdate(f)
		}
	}
}
