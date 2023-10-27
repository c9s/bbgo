package volume

import (
	"gonum.org/v1/gonum/floats"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type VWAPStream struct {
	*types.Float64Series
	window int
}

// The Volume Weighted Average Price (VWAP) provides the average price
// the asset has traded weighted by volume.
//
// VWAP = Sum(Closing * Volume) / Sum(Volume)
func VWAP(source v2.KLineSubscription, window int) *VWAPStream {
	var (
		pv     = v2.CloseMulVolume(source)
		volume = v2.Volumes(source)
		s      = &VWAPStream{
			Float64Series: types.NewFloat64Series(),
			window:        window,
		}
	)
	source.AddSubscriber(func(v types.KLine) {
		// var vwap = pv.Sum(window) / volume.Sum(s.window) // todo behaviour not the same?!
		var vwap = floats.Sum(pv.Slice.Tail(s.window)) / floats.Sum(volume.Slice.Tail(s.window))
		s.PushAndEmit(vwap)
	})
	return s
}

func VwapDefault(source v2.KLineSubscription) *VWAPStream {
	return VWAP(source, 14)
}

func (s *VWAPStream) Truncate() {
	s.Slice = s.Slice.Truncate(5000)
}
