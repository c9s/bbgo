package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// MINStream calculates the minimum value in a window from a float number stream.
type MINStream struct {
	*types.Float64Series

	window    int
	rawValues *types.Queue
}

func MIN(source types.Float64Source, window int) *MINStream {
	s := &MINStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		rawValues:     types.NewQueue(window),
	}

	s.Bind(source, s)
	return s
}

func (s *MINStream) Calculate(v float64) float64 {
	s.rawValues.Update(v)
	return types.Min(s.rawValues, s.window)
}

func (s *MINStream) Truncate() {
	s.Slice = types.ShrinkSlice(s.Slice, MaxSliceSize, TruncateSize)
}
