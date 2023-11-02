package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// 1 for Gain and -1 for Loss
type Coefficient int

const (
	GainCoefficient Coefficient = 1
	LossCoefficient Coefficient = -1
)

type GainLossStream struct {
	*types.Float64Series
	coefficient, gainSum float64
}

// Gain returns a derivative indicator that returns the gains in the underlying indicator in the last bar,
// if any. If the delta is negative, zero is returned
// Loss returns a derivative indicator that returns the losses in the underlying indicator in the last bar,
// if any. If the delta is positive, zero is returned
func GainLoss(source KLineSubscription, coff Coefficient) *GainLossStream {
	c := ClosePrices(source)
	s := &GainLossStream{
		Float64Series: types.NewFloat64Series(),
		coefficient:   float64(coff),
		gainSum:       0,
	}
	source.AddSubscriber(func(kLine types.KLine) {
		if source.Length() == 1 {
			s.PushAndEmit(Neutral)
		}

		var avgGain = s.gainSum / float64(source.Length())
		var gain = c.Last(1) - c.Last(0)
		if gain > 0 {
			s.gainSum += gain
		}
		avgGain = avgGain * (float64(source.Length()-1) + gain) / float64(source.Length())

		s.PushAndEmit(avgGain)
	})

	return s
}

// type cumulativeIndicator struct {
// 	Indicator
// 	window int
// 	mult   decimal.Decimal
// }

// // NewCumulativeGainsIndicator returns a derivative indicator which returns all gains made in a base indicator for a given
// // window.
// func NewCumulativeGainsIndicator(indicator Indicator, window int) Indicator {
// 	return cumulativeIndicator{
// 		Indicator: indicator,
// 		window:    window,
// 		mult:      decimal.NewFromInt(1),
// 	}
// }

// // NewCumulativeLossesIndicator returns a derivative indicator which returns all losses in a base indicator for a given
// // window.
// func NewCumulativeLossesIndicator(indicator Indicator, window int) Indicator {
// 	return cumulativeIndicator{
// 		Indicator: indicator,
// 		window:    window,
// 		mult:      decimal.NewFromInt(1).Neg(),
// 	}
// }

// func (ci cumulativeIndicator) Calculate(index int) decimal.Decimal {
// 	total := decimal.NewFromInt(0.0)

// 	for i := Max(1, index-(ci.window-1)); i <= index; i++ {
// 		diff := ci.Indicator.Calculate(i).Sub(ci.Indicator.Calculate(i - 1))
// 		if diff.Mul(ci.mult).GreaterThan(decimal.Zero) {
// 			total = total.Add(diff.Abs())
// 		}
// 	}

// 	return total
// }

// type percentChangeIndicator struct {
// 	Indicator
// }

// // NewPercentChangeIndicator returns a derivative indicator which returns the percent change (positive or negative)
// // made in a base indicator up until the given indicator
// func NewPercentChangeIndicator(indicator Indicator) Indicator {
// 	return percentChangeIndicator{indicator}
// }

// func (pgi percentChangeIndicator) Calculate(index int) decimal.Decimal {
// 	if index == 0 {
// 		return decimal.Zero
// 	}

// 	cp := pgi.Indicator.Calculate(index)
// 	cplast := pgi.Indicator.Calculate(index - 1)
// 	return cp.Div(cplast).Sub(decimal.NewFromInt(1))
// }
