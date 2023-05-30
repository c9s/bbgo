package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

type ATRStream struct {
	// embedded struct
	Float64Series

	// parameters
	types.IntervalWindow

	// private states
	rma *RMAStream

	window        int
	previousClose float64
}

func ATR2(source KLineSubscription, window int) *ATRStream {
	s := &ATRStream{
		Float64Series: NewFloat64Series(),
		window:        window,
	}
	s.rma = RMA2(s, window, true)

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
	s.slice.Push(trueRange)
	s.rma.EmitUpdate(trueRange)
}
