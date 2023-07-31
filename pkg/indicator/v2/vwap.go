package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfVWAP = 500_000

type VWAPStream struct {
	*types.Float64Series
	window          int
	rawCloseValues  *types.Queue
	rawVolumeValues *types.Queue
}

func VWAP(price types.Float64Source, volume types.Float64Source, window int) *VWAPStream {
	s := &VWAPStream{
		Float64Series:   types.NewFloat64Series(),
		window:          window,
		rawCloseValues:  types.NewQueue(window),
		rawVolumeValues: types.NewQueue(window),
	}
	price.OnUpdate(func(p float64) {
		s.rawCloseValues.Update(p)
		s.calculate()
	})
	volume.OnUpdate(func(v float64) {
		s.rawVolumeValues.Update(v)
		s.calculate()
	})
	return s
}

func (s *VWAPStream) calculate() float64 {
	vwap := s.rawCloseValues.Dot(s.rawVolumeValues) / s.rawVolumeValues.Sum(s.window)
	s.Slice.Push(vwap)
	s.EmitUpdate(vwap)
	return vwap
}

func (s *VWAPStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfVWAP)
}
