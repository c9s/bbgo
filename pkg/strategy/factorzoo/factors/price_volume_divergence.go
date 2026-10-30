package factorzoo

import (
	"time"

	"gonum.org/v1/gonum/stat"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// price volume divergence
// if the correlation of two time series gets smaller, they are diverging.
// so the negative value of the correlation of close price and volume is our alpha, PVD

var zeroTime time.Time

type KLineValueMapper func(k types.KLine) float64

type PVD struct {
	*types.Float64Series

	window  int
	Prices  *types.Queue
	Volumes *types.Queue
	EndTime time.Time
}

var _ types.SeriesExtend = &PVD{}

// NewPVD creates the price volume divergence factor stream from the given kline source.
// No value is emitted until window klines have been received.
func NewPVD(source indicatorv2.KLineSubscription, window int) *PVD {
	inc := &PVD{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		Prices:        types.NewQueue(window),
		Volumes:       types.NewQueue(window),
	}
	source.AddSubscriber(inc.PushK)
	return inc
}

func (inc *PVD) Update(price float64, volume float64) {
	inc.Prices.Update(price)
	inc.Volumes.Update(volume)
	if inc.Prices.Length() >= inc.window && inc.Volumes.Length() >= inc.window {
		divergence := -types.Correlation(inc.Prices, inc.Volumes, inc.window)
		inc.PushAndEmit(divergence)
		inc.Slice = types.ShrinkSlice(inc.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
	}
}

func (inc *PVD) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(types.KLineClosePriceMapper(k), types.KLineVolumeMapper(k))
	inc.EndTime = k.EndTime.Time()
}

func CalculateKLinesPVD(allKLines []types.KLine, window int) float64 {
	return pvd(types.MapKLinePrice(allKLines, types.KLineClosePriceMapper), types.MapKLinePrice(allKLines, types.KLineVolumeMapper), window)
}

func pvd(prices []float64, volumes []float64, window int) float64 {
	var end = len(prices) - 1
	if end == 0 {
		return prices[0]
	}

	divergence := -stat.Correlation(prices[end-window:end], volumes[end-window:end], nil)
	return divergence
}
