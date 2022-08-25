package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

/*
vwap implements the volume weighted average price (VWAP) indicator:

Volume Weighted Average Price (VWAP) Definition
- https://www.investopedia.com/terms/v/vwap.asp

Volume-Weighted Average Price (VWAP) Explained
- https://academy.binance.com/en/articles/volume-weighted-average-price-vwap-explained
*/
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

func (inc *VWAP) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *VWAP) Index(i int) float64 {
	length := len(inc.Values)
	if length == 0 || length-i-1 < 0 {
		return 0
	}

	return inc.Values[length-i-1]
}

func (inc *VWAP) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &VWAP{}

func (inc *VWAP) PushK(k types.KLine) {
	inc.Update(KLineTypicalPriceMapper(k), k.Volume.Float64())
}

func (inc *VWAP) CalculateAndUpdate(allKLines []types.KLine) {
	for _, k := range allKLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	inc.EmitUpdate(inc.Last())
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

func calculateVWAP(klines []types.KLine, priceF KLineValueMapper, window int) float64 {
	vwap := VWAP{IntervalWindow: types.IntervalWindow{Window: window}}
	for _, k := range klines {
		vwap.Update(priceF(k), k.Volume.Float64())
	}
	return vwap.Last()
}
