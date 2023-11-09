package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type VwmaStream struct {
	*types.Float64Series
	sma1   *SMAStream
	sma2   *SMAStream
	window int
}

// The Vwma function calculates the Volume Weighted Moving Average (VWMA)
// averaging the price data with an emphasis on volume, meaning areas
// with higher volume will have a greater weight.
//
// VWMA = Sum(Price * Volume) / Sum(Volume) for a given Period.
func Vwma(source KLineSubscription, window int) *VwmaStream {
	s := &VwmaStream{
		Float64Series: types.NewFloat64Series(),
		sma1:          SMA(CloseMulVolume(source), window),
		sma2:          SMA(Volumes(source), window),
		window:        window,
	}
	source.AddSubscriber(func(v types.KLine) {
		var vwma = s.sma1.Last(0) / s.sma2.Last(0)
		s.PushAndEmit(vwma)
	})
	return s
}

// The DefaultVwma function calculates VWMA with a period of 20.
func VwmaDefault(source KLineSubscription) *VwmaStream {
	return Vwma(source, 20)
}

func (s *VwmaStream) Calculate(_ float64) float64 {
	return s.Slice.Last(0)
}
