package drift

import (
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type DriftMA struct {
	types.SeriesBase
	drift *indicator.WeightedDrift
	ma1   types.UpdatableSeriesExtend
	ma2   types.UpdatableSeriesExtend
}

func (s *DriftMA) Update(value, weight float64) {
	s.ma1.Update(value)
	if s.ma1.Length() == 0 {
		return
	}
	s.drift.Update(s.ma1.Last(), weight)
	if s.drift.Length() == 0 {
		return
	}
	s.ma2.Update(s.drift.Last())
}

func (s *DriftMA) Last() float64 {
	return s.ma2.Last()
}

func (s *DriftMA) Index(i int) float64 {
	return s.ma2.Index(i)
}

func (s *DriftMA) Length() int {
	return s.ma2.Length()
}

func (s *DriftMA) ZeroPoint() float64 {
	return s.drift.ZeroPoint()
}

func (s *DriftMA) Clone() *DriftMA {
	out := DriftMA{
		drift: s.drift.Clone(),
		ma1:   types.Clone(s.ma1),
		ma2:   types.Clone(s.ma2),
	}
	out.SeriesBase.Series = &out
	return &out
}

func (s *DriftMA) TestUpdate(v, weight float64) *DriftMA {
	out := s.Clone()
	out.Update(v, weight)
	return out
}
