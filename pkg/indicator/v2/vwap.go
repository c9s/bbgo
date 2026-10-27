package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// VWAPStream implements the volume weighted average price (VWAP) indicator:
//
// Volume Weighted Average Price (VWAP) Definition
// - https://www.investopedia.com/terms/v/vwap.asp
//
// Volume-Weighted Average Price (VWAP) Explained
// - https://academy.binance.com/en/articles/volume-weighted-average-price-vwap-explained
//
// The Volume Weighted Average Price (VWAP) is a technical analysis indicator that is used to measure the average price of a security
// over a specified period of time, with the weighting factors determined by the volume of the security. It is calculated by taking the
// sum of the product of the price and volume for each trade, and then dividing that sum by the total volume of the security over the
// specified period of time. This resulting average is then plotted on the price chart as a line, which can be used to make predictions
// about future price movements. The VWAP is typically more accurate than other simple moving averages, as it takes into account the
// volume of the security, but may be less reliable in markets with low trading volume.
//
// The price of each kline is its typical price (high + low + close) / 3.
// A window of 0 computes the cumulative VWAP over all klines; a positive window
// computes a rolling VWAP over the last `window` klines.
type VWAPStream struct {
	*types.Float64Series

	window int

	prices, volumes floats.Slice

	WeightedSum float64
	VolumeSum   float64
}

// VWAP creates a VWAP stream over the typical price of the given klines.
// window == 0 means a cumulative (unbounded) VWAP.
func VWAP(source KLineSubscription, window int) *VWAPStream {
	s := &VWAPStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
	}

	source.AddSubscriber(func(k types.KLine) {
		s.PushAndEmit(s.calculate(types.KLineTypicalPriceMapper(k), k.Volume.Float64()))
		s.Truncate()
	})

	return s
}

func (s *VWAPStream) calculate(price, volume float64) float64 {
	if s.window > 0 {
		s.prices.Push(price)
		s.volumes.Push(volume)

		if len(s.prices) > s.window {
			s.WeightedSum -= s.prices[0] * s.volumes[0]
			s.VolumeSum -= s.volumes[0]
			s.prices = s.prices[1:]
			s.volumes = s.volumes[1:]
		}
	}

	s.WeightedSum += price * volume
	s.VolumeSum += volume

	return s.WeightedSum / s.VolumeSum
}

func (s *VWAPStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}
