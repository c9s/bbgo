package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

type PremiumSignalStream struct {
	*types.Float64Series

	price1, price2 float64
	premiumMargin  float64
}

func (s *PremiumSignalStream) calculatePremium() {
	if s.price1 == 0 || s.price2 == 0 {
		return
	}
	// calculate the premium type based on the prices
	var premiumRatio float64
	if math.Abs(s.price1-s.price2) < s.premiumMargin*s.price2 {
		premiumRatio = 0.0
	} else {
		premiumRatio = s.price1 / s.price2
	}
	// push and emit
	s.PushAndEmit(premiumRatio)
}

func PremiumSignal(
	priceStream1, priceStream2 *PriceStream,
	premiumMargin float64,
) *PremiumSignalStream {
	s := &PremiumSignalStream{
		Float64Series: types.NewFloat64Series(),
		premiumMargin: premiumMargin,
	}
	s.Subscribe(priceStream1, func(v float64) {
		s.price1 = v
		s.calculatePremium()
	})
	s.Subscribe(priceStream2, func(v float64) {
		s.price2 = v
		s.calculatePremium()
	})
	return s
}
