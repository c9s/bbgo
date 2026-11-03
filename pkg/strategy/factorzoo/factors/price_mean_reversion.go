package factorzoo

import (
	"time"

	"gonum.org/v1/gonum/stat"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// price mean reversion
// assume that the quotient of SMA over close price will dynamically revert into one.
// so this fraction value is our alpha, PMR

type PMR struct {
	*types.Float64Series

	window int

	// prices is the SMA input window
	prices *types.Queue

	// smaCount is the number of SMA values calculated so far
	smaCount int

	EndTime time.Time
}

var _ types.SeriesExtend = &PMR{}

// NewPMR creates the price mean reversion factor stream from the given kline source.
//
// It emits SMA(close, window) / close once window SMA values have been calculated,
// i.e. starting from the (2*window-1)th kline.
func NewPMR(source indicatorv2.KLineSubscription, window int) *PMR {
	inc := &PMR{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		prices:        types.NewQueue(window),
	}
	source.AddSubscriber(inc.PushK)
	return inc
}

func (inc *PMR) Update(price float64) {
	inc.prices.Update(price)
	if inc.prices.Length() < inc.window {
		return
	}

	sma := types.Mean(inc.prices)
	inc.smaCount++
	if inc.smaCount >= inc.window {
		reversion := sma / price
		inc.PushAndEmit(reversion)
		inc.Slice = types.ShrinkSlice(inc.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
	}
}

func (inc *PMR) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(types.KLineClosePriceMapper(k))
	inc.EndTime = k.EndTime.Time()
}

func CalculateKLinesPMR(allKLines []types.KLine, window int) float64 {
	return pmr(types.MapKLinePrice(allKLines, types.KLineClosePriceMapper), window)
}

func pmr(prices []float64, window int) float64 {
	var end = len(prices) - 1
	if end == 0 {
		return prices[0]
	}

	reversion := -stat.Mean(prices[end-window:end], nil) / prices[end]
	return reversion
}
