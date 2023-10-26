package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/indicator/v2/bst"
	"github.com/c9s/bbgo/pkg/types"
)

type MinValueStream struct {
	*types.Float64Series
	bst    *bst.Tree
	buffer []float64
	window int
}

func MinValue(source types.Float64Source, window int) *MinValueStream {
	s := &MinValueStream{
		Float64Series: types.NewFloat64Series(),
		bst:           bst.New(),
		buffer:        make([]float64, window),
		window:        window,
	}

	s.Bind(source, s)

	return s
}

func (s *MinValueStream) Calculate(v float64) float64 {
	s.bst.Insert(v)
	var i = s.Slice.Length()
	if i > 0 {
		s.bst.Remove(s.buffer[i%s.window])
	}

	s.buffer[i%s.window] = v
	return s.bst.Min().(float64)
}

func (s *MinValueStream) Truncate() {
	s.Slice = s.Slice.Truncate(s.window + 100)
}
