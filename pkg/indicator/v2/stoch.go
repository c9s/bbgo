package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

const DPeriod int = 3

// Stochastic Oscillator
// - https://www.investopedia.com/terms/s/stochasticoscillator.asp
//
// The Stochastic Oscillator is a technical analysis indicator that is used to identify potential overbought or oversold conditions
// in a security's price. It is calculated by taking the current closing price of the security and comparing it to the high and low prices
// over a specified period of time. This comparison is then plotted as a line on the price chart, with values above 80 indicating overbought
// conditions and values below 20 indicating oversold conditions. The Stochastic Oscillator can be used by traders to identify potential
// entry and exit points for trades, or to confirm other technical analysis signals. It is typically used in conjunction with other indicators
// to provide a more comprehensive view of the security's price.

//go:generate callbackgen -type StochStream
type StochStream struct {
	types.SeriesBase

	K, D floats.Slice

	window  int
	dPeriod int

	highPrices, lowPrices *PriceStream

	updateCallbacks []func(k, d float64)
}

// Stochastic Oscillator
func Stoch(source KLineSubscription, window, dPeriod int) *StochStream {
	highPrices := HighPrices(source)
	lowPrices := LowPrices(source)

	s := &StochStream{
		window:     window,
		dPeriod:    dPeriod,
		highPrices: highPrices,
		lowPrices:  lowPrices,
	}

	source.AddSubscriber(func(kLine types.KLine) {
		lowest := s.lowPrices.Slice.Tail(s.window).Min()
		highest := s.highPrices.Slice.Tail(s.window).Max()

		var k = 50.0
		var d = 0.0

		if highest != lowest {
			k = 100.0 * (kLine.Close.Float64() - lowest) / (highest - lowest)
		}

		s.K.Push(k)

		d = s.K.Tail(s.dPeriod).Mean()
		s.D.Push(d)
		s.EmitUpdate(k, d)
	})
	return s
}
