package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// ATRStream is a RMAStream
// This ATRStream calcualtes the ATR first, and then push it to the RMAStream
type ATRStream struct {
	// embedded struct
	Float64Series

	// private states
	previousClose float64
}

func ATR2(source KLineSubscription) *ATRStream {
	s := &ATRStream{
		Float64Series: NewFloat64Series(),
	}

	source.AddSubscriber(func(k types.KLine) {
		s.calculateAndPush(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	})
	return s
}

func (s *ATRStream) calculateAndPush(high, low, cls float64) {
	if s.previousClose == .0 {
		s.previousClose = cls
		return
	}

	trueRange := high - low
	hc := math.Abs(high - s.previousClose)
	lc := math.Abs(low - s.previousClose)
	if trueRange < hc {
		trueRange = hc
	}
	if trueRange < lc {
		trueRange = lc
	}

	s.previousClose = cls
	s.EmitUpdate(trueRange)
}
