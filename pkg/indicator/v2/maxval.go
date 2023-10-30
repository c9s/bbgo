package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/bst"
)

type MaxValueStream struct {
	*types.Float64Series
	bst    *bst.Tree
	buffer []float64
	window int
}

func MaxValue(source types.Float64Source, window int) *MaxValueStream {
	s := &MaxValueStream{
		Float64Series: types.NewFloat64Series(),
		bst:           bst.New(),
		buffer:        make([]float64, window),
		window:        window,
	}

	s.Bind(source, s)

	return s
}

func (s *MaxValueStream) Calculate(v float64) float64 {
	s.bst.Insert(v)
	var i = s.Slice.Length()
	if i > 0 {
		s.bst.Remove(s.buffer[i%s.window])
	}

	s.buffer[i%s.window] = v
	return s.bst.Max().(float64)
}

func (s *MaxValueStream) Truncate() {
	s.Slice = s.Slice.Truncate(s.window + 100)
}
