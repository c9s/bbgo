package supertrend

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

// LinGre is Linear Regression baseline
type LinGre struct {
	types.IntervalWindow
	baseLineSlope float64
}

// Update Linear Regression baseline slope
func (lg *LinGre) Update(klines []types.KLine) {
	if len(klines) < lg.Window {
		lg.baseLineSlope = 0
		return
	}

	var sumX, sumY, sumXSqr, sumXY float64 = 0, 0, 0, 0
	end := len(klines) - 1 // The last kline
	for i := end; i >= end-lg.Window+1; i-- {
		val := klines[i].GetClose().Float64()
		per := float64(end - i + 1)
		sumX += per
		sumY += val
		sumXSqr += per * per
		sumXY += val * per
	}
	length := float64(lg.Window)
	slope := (length*sumXY - sumX*sumY) / (length*sumXSqr - sumX*sumX)
	average := sumY / length
	endPrice := average - slope*sumX/length + slope
	startPrice := endPrice + slope*(length-1)
	lg.baseLineSlope = (length - 1) / (endPrice - startPrice)

	log.Debugf("linear regression baseline slope: %f", lg.baseLineSlope)
}

func (lg *LinGre) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if lg.Interval != interval {
		return
	}

	lg.Update(window)
}

func (lg *LinGre) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(lg.handleKLineWindowUpdate)
}

// GetSignal get linear regression signal
func (lg *LinGre) GetSignal() types.Direction {
	var lgSignal types.Direction = types.DirectionNone

	switch {
	case lg.baseLineSlope > 0:
		lgSignal = types.DirectionUp
	case lg.baseLineSlope < 0:
		lgSignal = types.DirectionDown
	}

	return lgSignal
}

// preloadLinGre preloads linear regression indicator
func (lg *LinGre) preload(kLineStore *bbgo.MarketDataStore) {
	if klines, ok := kLineStore.KLinesOfInterval(lg.Interval); ok {
		lg.Update((*klines)[0:])
	}
}
