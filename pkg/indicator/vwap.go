package indicator

import (
	"fmt"
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

/*
vwap implements the volume weighted average price (VWAP) indicator:

The basics of VWAP
- https://www.investopedia.com/terms/v/vwap.asp
- https://academy.binance.com/en/articles/volume-weighted-average-price-vwap-explained
*/
//go:generate callbackgen -type VWAP
type VWAP struct {
	types.IntervalWindow
	Values      Float64Slice
	WeightedSum float64
	VolumeSum   float64
	EndTime     time.Time

	UpdateCallbacks []func(value float64)
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
		volume := kLines[0].Volume

		inc.Values = []float64{price}
		inc.WeightedSum = price * volume
		inc.VolumeSum = volume
	} else {
		// from = len(inc.Values)

		// update ewma with the existing values
		for i := dataLen - 1; i > 0; i-- {
			var k = kLines[i]
			if k.EndTime.After(inc.EndTime) {
				from = i
			} else {
				break
			}
		}
	}

	for i := from; i < dataLen; i++ {
		var k = kLines[i]

		// add next
		inc.WeightedSum += priceF(k) * k.Volume
		inc.VolumeSum += k.Volume

		// drop first
		if i >= inc.Window {
			var dropK = kLines[i-inc.Window]
			inc.WeightedSum -= priceF(dropK) * dropK.Volume
			inc.VolumeSum -= dropK.Volume
		}

		vwap := inc.WeightedSum / inc.VolumeSum
		inc.Values.Push(vwap)
		inc.EndTime = k.EndTime
		inc.EmitUpdate(vwap)
	}

	// verify the result of accumulated vwap with sliding window method
	var index = len(kLines) - 1
	var recentK = kLines[index-inc.Window+1 : index+1]

	v2, err := calculateVWAP(recentK, KLineTypicalPriceMapper)
	if err != nil {
		log.WithError(err).Error("VWAP error")
		return
	}

	v1 := inc.Values[index]
	diff := math.Abs(v1 - v2)
	if diff > 1e-5 {
		log.Warnf("ACCUMULATED %s VWAP (%d) %f != VWAP %f", inc.Interval, inc.Window, v1, v2)
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

func calculateVWAP(kLines []types.KLine, priceF KLinePriceMapper) (float64, error) {
	length := len(kLines)
	if length == 0 {
		return 0.0, fmt.Errorf("insufficient elements for calculating VWAP")
	}

	weightedSum := 0.0
	volumeSum := 0.0

	// TODO: move the following func to calculateAndUpdate and support sliding window method
	update := func(price float64, volume float64) {
		weightedSum += price * volume
		volumeSum += volume
	}

	for _, k := range kLines {
		update(priceF(k), k.Volume)
	}

	avg := weightedSum / volumeSum
	return avg, nil
}
