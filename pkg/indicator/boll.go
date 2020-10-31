package indicator

import (
	"time"

	"gonum.org/v1/gonum/stat"

	"github.com/c9s/bbgo/pkg/types"
)

/*
BOLL implements the bollinger indicator:

The Basics of Bollinger Bands
- https://www.investopedia.com/articles/technical/102201.asp

Bollinger Bands
- https://www.investopedia.com/terms/b/bollingerbands.asp

Bollinger Bands Technical indicator guide:
- https://www.fidelity.com/learning-center/trading-investing/technical-analysis/technical-indicator-guide/bollinger-bands
*/
type BOLL struct {
	types.IntervalWindow

	// times of Std, generally it's 2
	K float64

	SMA      Float64Slice
	StdDev   Float64Slice
	UpBand   Float64Slice
	DownBand Float64Slice

	EndTime time.Time
}

func (inc *BOLL) LastSMA() float64 {
	return inc.SMA[len(inc.SMA)-1]
}

func (inc *BOLL) calculateAndUpdate(kLines []types.KLine) {
	if len(kLines) < inc.Window {
		return
	}

	var index = len(kLines) - 1
	var kline = kLines[index]

	if inc.EndTime != zeroTime && kline.EndTime.Before(inc.EndTime) {
		return
	}

	var recentK = kLines[index-(inc.Window-1) : index+1]
	var sma = calculateSMA(recentK)
	inc.SMA.Push(sma)

	var prices []float64
	for _, k := range recentK {
		prices = append(prices, k.Close)
	}

	var std = stat.StdDev(prices, nil)
	inc.StdDev.Push(std)

	var upBand = sma + inc.K*std
	inc.UpBand.Push(upBand)

	var downBand = sma - inc.K*std
	inc.DownBand.Push(downBand)

	// update end time
	inc.EndTime = kLines[index].EndTime
}

func (inc *BOLL) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	if inc.EndTime != zeroTime && inc.EndTime.Before(inc.EndTime) {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *BOLL) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
