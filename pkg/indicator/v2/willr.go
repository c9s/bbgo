package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Williams %R (Williams Percent Range)
// - https://www.investopedia.com/terms/w/williamsr.asp
//
// The Williams %R is a momentum indicator that measures overbought and oversold levels,
// comparing a security's close to the high-low range over a specific period (typically 14 periods).
// Formula:
// %R = (Highest High - Close) / (Highest High - Lowest Low) * -100
// Values range from 0 to -100.
// Readings between 0 and -20 are considered overbought, while readings between -80 and -100 are considered oversold.

//go:generate callbackgen -type WillRStream
type WillRStream struct {
	types.SeriesBase

	Values floats.Slice

	window int

	highPrices, lowPrices *PriceStream

	updateCallbacks []func(value float64)
}

// Williams %R
func WillR(source KLineSubscription, window int) *WillRStream {
	checkWindow(window)

	highPrices := HighPrices(source)
	lowPrices := LowPrices(source)

	s := &WillRStream{
		window:     window,
		highPrices: highPrices,
		lowPrices:  lowPrices,
	}
	s.SeriesBase.Series = &s.Values

	source.AddSubscriber(func(kLine types.KLine) {
		lowest := s.lowPrices.Slice.Tail(s.window).Min()
		highest := s.highPrices.Slice.Tail(s.window).Max()

		var willr = -50.0

		if highest != lowest {
			willr = -100.0 * (highest - kLine.Close.Float64()) / (highest - lowest)
		}

		s.Values.Push(willr)
		s.Values = generalTruncate(s.Values)
		s.EmitUpdate(willr)
	})
	return s
}
