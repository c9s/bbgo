package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

type MultiplyStream struct {
	*types.Float64Series
	a, b floats.Slice
}

func Multiply(a, b types.Float64Source) *MultiplyStream {
	s := &MultiplyStream{
		Float64Series: types.NewFloat64Series(),
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

func (s *MultiplyStream) calculate() {
	if s.a.Length() != s.b.Length() {
		return
	}

	if s.a.Length() > s.Slice.Length() {
		var numNewElems = s.a.Length() - s.Slice.Length()
		var tailA = s.a.Tail(numNewElems)
		var tailB = s.b.Tail(numNewElems)
		var tailC = tailA.Mul(tailB)
		for _, f := range tailC {
			s.Slice.Push(f)
			s.EmitUpdate(f)
		}
	}
}
