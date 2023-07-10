package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

type PivotHighStream struct {
	*types.Float64Series
	rawValues           floats.Slice
	window, rightWindow int
}

func PivotHigh2(source types.Float64Source, window, rightWindow int) *PivotHighStream {
	s := &PivotHighStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		rightWindow:   rightWindow,
	}

	s.Subscribe(source, func(x float64) {
		s.rawValues.Push(x)
		if low, ok := s.calculatePivotHigh(s.rawValues, s.window, s.rightWindow); ok {
			s.PushAndEmit(low)
		}
	})
	return s
}

func (s *PivotHighStream) calculatePivotHigh(highs floats.Slice, left, right int) (float64, bool) {
	return floats.FindPivot(highs, left, right, func(a, pivot float64) bool {
		return a < pivot
	})
}
