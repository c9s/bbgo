package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// ROCStream calculates Price Rate of Change (ROC).
//
// ROC = ((price - price_n_periods_ago) / price_n_periods_ago) * 100
type ROCStream struct {
	*types.Float64Series

	window int
	prices floats.Slice
}

func ROC2(source KLineSubscription, window int) *ROCStream {
	checkWindow(window)

	s := &ROCStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
	}

	ClosePrices(source).OnUpdate(func(price float64) {
		s.calculateAndPush(price)
	})
	return s
}

func (s *ROCStream) calculateAndPush(price float64) {
	s.prices.Push(price)
	if s.prices.Length() <= s.window {
		return
	}

	prevPrice := s.prices[s.prices.Length()-1-s.window]
	var roc float64
	if prevPrice != 0 {
		roc = ((price - prevPrice) / prevPrice) * 100.0
	}

	s.PushAndEmit(roc)
	s.Truncate()
}

func (s *ROCStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
	if s.prices.Length() > MaxSliceSize+s.window {
		s.prices = s.prices[s.prices.Length()-MaxSliceSize:]
	}
}
