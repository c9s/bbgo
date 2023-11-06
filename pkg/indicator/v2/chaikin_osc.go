package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// The DefaultChaikinOscillator function calculates Chaikin
// Oscillator with the most frequently used fast and short
// periods, 3 and 10.
func NewChaikinOscillatorDefault(source KLineSubscription) *ChaikinOscillatorStream {
	return ChaikinOscillator(source, 3, 10)
}

type ChaikinOscillatorStream struct {
	// embedded structs
	*types.Float64Series
}

func ChaikinOscillator(source KLineSubscription, slow, fast int) *ChaikinOscillatorStream {
	var (
		accDist = AccumulationDistribution(source)
		diff    = Subtract(EWMA2(accDist, slow), EWMA2(accDist, fast))
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
	s.Slice = s.Slice.Truncate(MaxNumOfMA)
}
