package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

type PremiumStream struct {
	*types.Float64Series

	priceSlice1, priceSlice2 floats.Slice
	premiumMargin            float64
}

func (s *PremiumStream) Truncate() {
	s.priceSlice1 = types.ShrinkSlice(s.priceSlice1, MaxSliceSize, TruncateSize)
	s.priceSlice2 = types.ShrinkSlice(s.priceSlice2, MaxSliceSize, TruncateSize)
}

func (s *PremiumStream) calculatePremium() {
	if s.priceSlice1.Length() != s.priceSlice2.Length() {
		return
	}
	// calculate the premium type based on the prices
	price1 := s.priceSlice1.Last(0)
	price2 := s.priceSlice2.Last(0)
	var premiumRatio float64
	if math.Abs(price1-price2) < s.premiumMargin*price2 {
		premiumRatio = 0.0
	} else {
		premiumRatio = price1 / price2
	}
	// push and emit
	s.PushAndEmit(premiumRatio)
}

func Premium(
	priceStream1, priceStream2 *PriceStream,
	premiumMargin float64,
) *PremiumStream {
	s := &PremiumStream{
		Float64Series: types.NewFloat64Series(),
		premiumMargin: premiumMargin,
	}
	s.Subscribe(priceStream1, func(v float64) {
		s.priceSlice1.Append(v)
		s.calculatePremium()
	})
	s.Subscribe(priceStream2, func(v float64) {
		s.priceSlice2.Append(v)
		s.calculatePremium()
	})
	return s
}
