package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// VWMAStream implements the volume weighted moving average (VWMA) indicator:
//
// Calculation:
//
//	pv = element-wise multiplication of close prices and volumes
//	VWMA = SMA(pv, window) / SMA(volumes, window)
//
// Volume Weighted Moving Average
// - https://www.motivewave.com/studies/volume_weighted_moving_average.htm
//
// The Volume Weighted Moving Average (VWMA) is a technical analysis indicator that is used to smooth price data and reduce the lag
// associated with traditional moving averages. It is calculated by taking the weighted moving average of the input data, with the
// weighting factors determined by the volume of the security. This resulting average is then plotted on the price chart as a line,
// which can be used to make predictions about future price movements. The VWMA is typically more accurate than other simple moving
// averages, as it takes into account the volume of the security, but may be less reliable in markets with low trading volume.
//
// Before window klines have been collected, the value is the VWMA of the available klines.
type VWMAStream struct {
	*types.Float64Series

	PriceVolumeSMA *SMAStream
	VolumeSMA      *SMAStream

	priceVolumes, volumes *types.Float64Series
}

// VWMA creates the volume weighted moving average stream from the kline source (close price and volume)
func VWMA(source KLineSubscription, window int) *VWMAStream {
	checkWindow(window)

	priceVolumes := types.NewFloat64Series()
	volumes := types.NewFloat64Series()

	s := &VWMAStream{
		Float64Series:  types.NewFloat64Series(),
		PriceVolumeSMA: SMA(priceVolumes, window),
		VolumeSMA:      SMA(volumes, window),
		priceVolumes:   priceVolumes,
		volumes:        volumes,
	}

	source.AddSubscriber(func(k types.KLine) {
		s.Update(k.Close.Float64(), k.Volume.Float64())
	})
	return s
}

// Update pushes the price and volume into the stream.
func (s *VWMAStream) Update(price, volume float64) {
	s.priceVolumes.PushAndEmit(price * volume)
	s.volumes.PushAndEmit(volume)

	s.PushAndEmit(s.PriceVolumeSMA.Last(0) / s.VolumeSMA.Last(0))
	s.Truncate()
}

func (s *VWMAStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
	s.priceVolumes.Slice = generalTruncate(s.priceVolumes.Slice)
	s.volumes.Slice = generalTruncate(s.volumes.Slice)
}
