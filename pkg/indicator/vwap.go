package indicator

import (
	"fmt"
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
	Values  Float64Slice
	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *VWAP) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		return
	}

	var index = len(kLines) - 1
	var kline = kLines[index]

	if inc.EndTime != zeroTime && kline.EndTime.Before(inc.EndTime) {
		return
	}

	var recentK = kLines[len(kLines)-1-(inc.Window-1) : index+1]

	vwap, err := calculateVWAP(recentK, KLineTypicalPriceMapper)
	if err != nil {
		log.WithError(err).Error("VWAP error")
		return
	}

	inc.Values.Push(vwap)
	inc.EndTime = kLines[index].EndTime
	inc.EmitUpdate(vwap)
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

	for _, k := range kLines {
		weightedSum += priceF(k) * k.Volume
		volumeSum += k.Volume
	}

	avg := weightedSum / volumeSum
	return avg, nil
}
