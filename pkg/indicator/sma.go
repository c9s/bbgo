package indicator

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfSMA = 1_000
const MaxNumOfSMATruncateSize = 500

var zeroTime time.Time

//go:generate callbackgen -type SMA
type SMA struct {
	types.IntervalWindow
	Values  types.Float64Slice
	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *SMA) Last() float64 {
	return inc.Values[len(inc.Values)-1]
}

func (inc *SMA) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		return
	}

	var index = len(kLines) - 1
	var kline = kLines[index]

	if inc.EndTime != zeroTime && kline.EndTime.Before(inc.EndTime) {
		return
	}

	var recentK = kLines[index-(inc.Window-1) : index+1]

	sma, err := calculateSMA(recentK, inc.Window, KLineClosePriceMapper)
	if err != nil {
		log.WithError(err).Error("SMA error")
		return
	}
	inc.Values.Push(sma)

	if len(inc.Values) > MaxNumOfSMA {
		inc.Values = inc.Values[MaxNumOfSMATruncateSize:]
	}

	inc.EndTime = kLines[index].EndTime

	inc.EmitUpdate(sma)
}

func (inc *SMA) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *SMA) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateSMA(kLines []types.KLine, window int, priceF KLinePriceMapper) (float64, error) {
	length := len(kLines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating SMA with window = %d", window)
	}

	sum := 0.0
	for _, k := range kLines {
		sum += priceF(k)
	}

	avg := sum / float64(window)
	return avg, nil
}
