package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

type CrossType float64

const (
	CrossOver  CrossType = 1.0
	CrossUnder CrossType = -1.0
)

// CrossStream subscribes 2 upstreams, and calculate the cross signal
type CrossStream struct {
	*types.Float64Series

	a, b floats.Slice
}

// Cross creates the CrossStream object:
//
// cross := Cross(fastEWMA, slowEWMA)
func Cross(a, b types.Float64Source) *CrossStream {
	s := &CrossStream{
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

func (s *CrossStream) calculate() {
	if s.a.Length() != s.b.Length() {
		return
	}

	current := s.a.Last(0) - s.b.Last(0)
	previous := s.a.Last(1) - s.b.Last(1)

	if previous == 0.0 {
		return
	}

	// cross over or cross under
	if current*previous < 0 {
		if current > 0 {
			s.PushAndEmit(float64(CrossOver))
		} else {
			s.PushAndEmit(float64(CrossUnder))
		}
	}
}
