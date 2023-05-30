package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

type RMAStream struct {
	// embedded structs
	Float64Series

	// config fields
	types.IntervalWindow
	Adjust bool

	counter  int
	sum, tmp float64
}

func RMA2(source Float64Source, iw types.IntervalWindow) *RMAStream {
	s := &RMAStream{
		IntervalWindow: iw,
	}

	s.SeriesBase.Series = s.slice

	if sub, ok := source.(Float64Subscription); ok {
		sub.AddSubscriber(s.calculateAndPush)
	} else {
		source.OnUpdate(s.calculateAndPush)
	}

	return s
}

func (s *RMAStream) calculateAndPush(v float64) {
	v2 := s.calculate(v)
	s.slice.Push(v2)
	s.EmitUpdate(v2)
}

func (s *RMAStream) calculate(x float64) float64 {
	lambda := 1 / float64(s.Window)
	if s.counter == 0 {
		s.sum = 1
		s.tmp = x
	} else {
		if s.Adjust {
			s.sum = s.sum*(1-lambda) + 1
			s.tmp = s.tmp + (x-s.tmp)/s.sum
		} else {
			s.tmp = s.tmp*(1-lambda) + x*lambda
		}
	}
	s.counter++

	if s.counter < s.Window {
		s.slice.Push(0)
	}

	s.slice.Push(s.tmp)

	if len(s.slice) > MaxNumOfRMA {
		s.slice = s.slice[MaxNumOfRMATruncateSize-1:]
	}

	return s.tmp
}
