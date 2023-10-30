package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Williams R. Determine overbought and oversold.

// Developed by Larry Williams, Williams %R is a momentum indicator that is
// the inverse of the Fast Stochastic Oscillator. Also referred to as %R,
// Williams %R reflects the level of the close relative to the highest high
// for the look-back period. In contrast, the Stochastic Oscillator reflects
// the level of the close relative to the lowest low. %R corrects for the
// inversion by multiplying the raw value by -100. As a result, the Fast
// Stochastic Oscillator and Williams %R produce the exact same lines, but
// with different scaling. Williams %R oscillates from 0 to -100; readings
// from 0 to -20 are considered overbought, while readings from -80 to -100
// are considered oversold. Unsurprisingly, signals derived from the Stochastic
// Oscillator are also applicable to Williams %R.
//
//	https://school.stockcharts.com/doku.php?id=technical_indicators:williams_r
//	https://www.investopedia.com/terms/w/williamsr.asp
//
// WR = (Highest High - Closing) / (Highest High - Lowest Low) * -100.
//
// Buy when -80 and below. Sell when -20 and above.
type WilliamsRStream struct {
	// embedded structs
	*types.Float64Series
	min *MinValueStream
	max *MaxValueStream
}

func WilliamsR(source KLineSubscription, window int) *WilliamsRStream {
	s := &WilliamsRStream{
		Float64Series: types.NewFloat64Series(),
		min:           MinValue(LowPrices(source), window),
		max:           MaxValue(HighPrices(source), window),
	}
	source.AddSubscriber(func(v types.KLine) {

		highestHigh := s.max.Last(0)
		lowestLow := s.min.Last(0)
		var w = (highestHigh - v.Close.Float64()) / (highestHigh - lowestLow) * -100

		s.PushAndEmit(w)
	})

	return s
}

func (s *WilliamsRStream) Truncate() {
	s.Slice = s.Slice.Truncate(5000)
}
