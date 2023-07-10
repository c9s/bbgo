package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// vwap implements the volume weighted average price (VWAP) indicator:
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

//go:generate callbackgen -type VWAP
type VWAP struct {
	types.SeriesBase
	types.IntervalWindow
	Values      floats.Slice
	Prices      floats.Slice
	Volumes     floats.Slice
	WeightedSum float64
	VolumeSum   float64

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *VWAP) Update(price, volume float64) {
	if len(inc.Prices) == 0 {
		inc.SeriesBase.Series = inc
	}
	inc.Prices.Push(price)
	inc.Volumes.Push(volume)

	if inc.Window != 0 && len(inc.Prices) > inc.Window {
		popIndex := len(inc.Prices) - inc.Window - 1
		inc.WeightedSum -= inc.Prices[popIndex] * inc.Volumes[popIndex]
		inc.VolumeSum -= inc.Volumes[popIndex]
	}

	inc.WeightedSum += price * volume
	inc.VolumeSum += volume

	vwap := inc.WeightedSum / inc.VolumeSum
	inc.Values.Push(vwap)
}

func (inc *VWAP) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *VWAP) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *VWAP) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &VWAP{}

func (inc *VWAP) PushK(k types.KLine) {
	inc.Update(types.KLineTypicalPriceMapper(k), k.Volume.Float64())
}

func (inc *VWAP) CalculateAndUpdate(allKLines []types.KLine) {
	for _, k := range allKLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	inc.EmitUpdate(inc.Last(0))
	inc.EndTime = allKLines[len(allKLines)-1].EndTime.Time()
}

func (inc *VWAP) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *VWAP) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateVWAP(klines []types.KLine, priceF types.KLineValueMapper, window int) float64 {
	vwap := VWAP{IntervalWindow: types.IntervalWindow{Window: window}}
	for _, k := range klines {
		vwap.Update(priceF(k), k.Volume.Float64())
	}
	return vwap.Last(0)
}
