package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfRMA = 1000
const MaxNumOfRMATruncateSize = 500

type RMAStream struct {
	// embedded structs
	*types.Float64Series

	// config fields
	Adjust bool

	window        int
	counter       int
	sum, previous float64
}

func RMA2(source types.Float64Source, window int, adjust bool) *RMAStream {
	s := &RMAStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		Adjust:        adjust,
	}

	s.Bind(source, s)
	return s
}

func (s *RMAStream) Calculate(x float64) float64 {
	lambda := 1 / float64(s.window)
	tmp := 0.0
	if s.counter == 0 {
		s.sum = 1
		tmp = x
	} else {
		if s.Adjust {
			s.sum = s.sum*(1-lambda) + 1
			tmp = s.previous + (x-s.previous)/s.sum
		} else {
			tmp = s.previous*(1-lambda) + x*lambda
		}
	}
	s.counter++

	if s.counter < s.window {
		// we can use x, but we need to use 0. to make the same behavior as the result from python pandas_ta
		s.Slice.Push(0)
	}

	s.Slice.Push(tmp)
	s.previous = tmp

	return tmp
}

func (s *RMAStream) Truncate() {
	if len(s.Slice) > MaxNumOfRMA {
		s.Slice = s.Slice[MaxNumOfRMATruncateSize-1:]
	}
}
