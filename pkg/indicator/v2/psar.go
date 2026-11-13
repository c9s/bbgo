package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// PSARStream implements the Parabolic SAR (Stop and Reverse) / SAR indicator
//
// Refer: https://www.investopedia.com/terms/p/parabolicindicator.asp
// The parabolic SAR indicator, developed by J. Wells Wilder, is used by traders to determine
// trend direction and potential reversals in price. The indicator uses a trailing stop and
// reverse method called "SAR," or stop and reverse, to identify suitable exit and entry points.
// Traders also refer to the indicator as to the parabolic stop and reverse, parabolic SAR, or PSAR.
//
// The parabolic SAR indicator appears on a chart as a series of dots, either above or below an asset's
// price, depending on the direction the price is moving. A dot is placed below the price when it is
// trending upward, and above the price when it is trending downward.
//
// The first kline only seeds the high/low history, so the first SAR value is
// emitted on the second kline.
type PSARStream struct {
	*types.Float64Series

	window int

	High, Low *types.Queue

	// AF is the acceleration factor
	AF float64

	// EP is the extreme point
	EP float64

	// Falling is true when the SAR is in a downtrend (dots above the price)
	Falling bool
}

func PSAR(source KLineSubscription, window int) *PSARStream {
	checkWindow(window)

	s := &PSARStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		High:          types.NewQueue(window),
		Low:           types.NewQueue(window),
		AF:            0.02,
	}

	source.AddSubscriber(func(k types.KLine) {
		s.update(k.High.Float64(), k.Low.Float64())
	})

	return s
}

func (s *PSARStream) falling() bool {
	up := s.High.Last(0) - s.High.Index(1)
	dn := s.Low.Index(1) - s.Low.Last(0)
	return (dn > up) && (dn > 0)
}

func (s *PSARStream) update(high, low float64) {
	if s.High.Length() == 0 {
		s.High.Update(high)
		s.Low.Update(low)
		return
	}

	isFirst := s.High.Length() < s.window
	s.High.Update(high)
	s.Low.Update(low)

	var sar float64
	if isFirst {
		s.Falling = s.falling()
		if s.Falling {
			sar = s.High.Index(1)
			s.EP = s.Low.Index(1)
		} else {
			sar = s.Low.Index(1)
			s.EP = s.High.Index(1)
		}
	} else {
		ppsar := s.Last(0)
		if s.Falling { // falling formula
			psar := ppsar - s.AF*(ppsar-s.EP)
			h := s.High.Shift(1).Highest(2)
			sar = math.Max(psar, h)
			if low < s.EP {
				s.EP = low
				if s.AF <= 0.18 {
					s.AF += 0.02
				}
			}
			if high > psar { // reverse
				s.AF = 0.02
				sar = s.EP
				s.EP = high
				s.Falling = false
			}
		} else { // rising formula
			psar := ppsar + s.AF*(s.EP-ppsar)
			l := s.Low.Shift(1).Lowest(2)
			sar = math.Min(psar, l)
			if high > s.EP {
				s.EP = high
				if s.AF <= 0.18 {
					s.AF += 0.02
				}
			}
			if low < psar { // reverse
				s.AF = 0.02
				sar = s.EP
				s.EP = low
				s.Falling = true
			}
		}
	}

	s.PushAndEmit(sar)
	s.Truncate()
}

func (s *PSARStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}
