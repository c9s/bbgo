package indicator

import (
	"time"

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
	types.IntervalWindow
	Values      types.Float64Slice
	WeightedSum float64
	VolumeSum   float64
	EndTime     time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *VWAP) calculateVWAP(kLines []types.KLine, priceF KLinePriceMapper) (vwap float64) {
	for i, k := range kLines {
		inc.update(k, priceF, 1.0) // add kline

		// if window size is not zero, then we do not apply sliding window method
		if inc.Window != 0 && len(inc.Values) >= inc.Window {
			inc.update(kLines[i-inc.Window], priceF, -1.0) // pop kline
		}
		vwap = inc.WeightedSum / inc.VolumeSum
		inc.Values.Push(vwap)
	}

	return vwap
}

func (inc *VWAP) update(kLine types.KLine, priceF KLinePriceMapper, multiplier float64) {
	// multiplier = 1 or -1
	price := priceF(kLine)
	volume := kLine.Volume.Float64()

	inc.WeightedSum += multiplier * price * volume
	inc.VolumeSum += multiplier * volume
}

func (inc *VWAP) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		return
	}

	var priceF = KLineTypicalPriceMapper
	var dataLen = len(kLines)

	// init the values from the kline data
	var from = 1
	if len(inc.Values) == 0 {
		// for the first value, we should use the close price
		price := priceF(kLines[0])
		volume := kLines[0].Volume.Float64()

		inc.Values = []float64{price}
		inc.WeightedSum = price * volume
		inc.VolumeSum = volume
	} else {
		// update vwap with the existing values
		for i := dataLen - 1; i > 0; i-- {
			var k = kLines[i]
			if k.EndTime.After(inc.EndTime) {
				from = i
			} else {
				break
			}
		}
	}

	// update vwap
	for i := from; i < dataLen; i++ {
		inc.update(kLines[i], priceF, 1.0) // add kline

		if i >= inc.Window {
			inc.update(kLines[i-inc.Window], priceF, -1.0) // pop kline
		}
		vwap := inc.WeightedSum / inc.VolumeSum

		inc.Values.Push(vwap)
		inc.EmitUpdate(vwap)

		inc.EndTime = kLines[i].EndTime.Time()
	}
}

func (inc *VWAP) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *VWAP) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
