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
	checkWindow(window)

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
	if s.counter == 0 {
		s.sum = 1
		s.previous = x
	} else {
		if s.Adjust {
			s.sum = s.sum*(1-lambda) + 1
			s.previous = s.previous + (x-s.previous)/s.sum
		} else {
			s.previous = s.previous*(1-lambda) + x*lambda
		}
	}
	s.counter++

	return s.previous
}

func (s *RMAStream) Truncate() {
	if len(s.Slice) > MaxNumOfRMA {
		s.Slice = s.Slice[MaxNumOfRMATruncateSize-1:]
	}
}

func checkWindow(window int) {
	if window == 0 {
		panic("window can not be zero")
	}
}
