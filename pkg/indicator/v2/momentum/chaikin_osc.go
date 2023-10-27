package momentum

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/indicator/v2/trend"
	"github.com/c9s/bbgo/pkg/indicator/v2/volume"
	"github.com/c9s/bbgo/pkg/types"
)

// The DefaultChaikinOscillator function calculates Chaikin
// Oscillator with the most frequently used fast and short
// periods, 3 and 10.
func NewChaikinOscillatorDefault(source v2.KLineSubscription) *ChaikinOscillatorStream {
	return ChaikinOscillator(source, 3, 10)
}

type ChaikinOscillatorStream struct {
	// embedded structs
	*types.Float64Series
}

func ChaikinOscillator(source v2.KLineSubscription, slow, fast int) *ChaikinOscillatorStream {
	var (
		accDist = volume.AccumulationDistribution(source)
		diff    = v2.Subtract(trend.EWMA2(accDist, slow), trend.EWMA2(accDist, fast))
		s       = &ChaikinOscillatorStream{
			Float64Series: types.NewFloat64Series(),
		}
	)
	source.AddSubscriber(func(kLine types.KLine) {
		s.PushAndEmit(diff.Last(0))
	})

	return s
}

func (s *ChaikinOscillatorStream) Truncate() {
	s.Slice = s.Slice.Truncate(5000)
}
