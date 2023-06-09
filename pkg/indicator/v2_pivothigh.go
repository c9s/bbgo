package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
)

type PivotHighStream struct {
	*Float64Series
	rawValues           floats.Slice
	window, rightWindow int
}

func PivotHigh2(source Float64Source, window, rightWindow int) *PivotHighStream {
	s := &PivotHighStream{
		Float64Series: NewFloat64Series(),
		window:        window,
		rightWindow:   rightWindow,
	}

	s.Subscribe(source, func(x float64) {
		s.rawValues.Push(x)
		if low, ok := calculatePivotHigh(s.rawValues, s.window, s.rightWindow); ok {
			s.PushAndEmit(low)
		}
	})
	return s
}
