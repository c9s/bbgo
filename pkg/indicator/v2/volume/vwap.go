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
		hlc3v  = v2.CloseMulVolume(source)
		volume = v2.Volumes(source)
		s      = &VWAPStream{
			Float64Series: types.NewFloat64Series(),
			window:        window,
		}
	)
	source.AddSubscriber(func(v types.KLine) {
		// var vwap = hlc3v.Sum(window) / volume.Sum(s.window) // todo behaviour not the same?!
		var vwap = floats.Sum(hlc3v.Slice.Tail(s.window)) / floats.Sum(volume.Slice.Tail(s.window))
		s.PushAndEmit(vwap)
	})
	return s
}
