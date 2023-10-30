package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// This TRStream calculates the ATR first
type TRStream struct {
	// embedded struct
	*types.Float64Series

	// private states
	previousClose float64
}

func TR2(source KLineSubscription) *TRStream {
	s := &TRStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(k types.KLine) {
		s.calculateAndPush(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	})
	return s
}

func (s *TRStream) calculateAndPush(high, low, cls float64) {
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

	s.PushAndEmit(trueRange)
}
