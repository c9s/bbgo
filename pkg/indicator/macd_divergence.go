package indicator

import (
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datatype/floats"
)

type PivotFunc func(a, pivot float64) bool

func PivotHighFinder(a, pivot float64) bool {
	return pivot > 0 && pivot > a
}

func PivotLowFinder(a, pivot float64) bool {
	return pivot < 0 && pivot < a
}

// MacdMomentum is used for finding up/down momentum
type MacdMomentum struct {
	*MACDConfig
	PivotWindow int `json:"pivotWindow"`

	macd *MACD
}

func (d *MacdMomentum) Init(standardIndicator *bbgo.StandardIndicatorSet) {
	d.macd = standardIndicator.MACD(d.IntervalWindow, d.ShortPeriod, d.LongPeriod)

	// Add an updater for updating the divergence
	d.macd.OnUpdate(func(macd, signal, histogram float64) {
		logrus.Infof("MACD %+v: macd: %f, signal: %f histogram: %f", d.macd.IntervalWindow, macd, signal, histogram)
		d.detect()
	})
	d.detect()
}

func (d *MacdMomentum) checkBottom() {

}

func (d *MacdMomentum) findHistogramPivots(finder PivotFunc) floats.Slice {
	pivotWindow := d.PivotWindow
	if pivotWindow == 0 {
		pivotWindow = 3
	}

	if len(d.macd.Histogram) < pivotWindow*2 {
		logrus.Warnf("histogram values is not enough for finding pivots, length=%d", len(d.macd.Histogram))
		return nil
	}

	var histogram = d.macd.Histogram
	var pivots floats.Slice
	for i := pivotWindow; i > 0 && i < len(histogram); i++ {
		// find positive histogram and the top
		if pivot, ok := floats.FindPivot(histogram[0:i], pivotWindow, pivotWindow, finder); ok {
			pivots = append(pivots, pivot)
		}
	}

	return pivots
}

func (d *MacdMomentum) detect() bool {
	// always reset the top divergence to false
	topDivergence := false

	var histogramPivots = d.findHistogramPivots(PivotHighFinder)
	if len(histogramPivots) == 0 {
		return false
	}

	logrus.Infof("histogram pivots: %+v", histogramPivots)

	// take the last 2-3 pivots to check if there is a divergence
	if len(histogramPivots) < 3 {
		return false
	}

	histogramPivots = histogramPivots[len(histogramPivots)-3:]
	minDiff := 0.01
	for i := len(histogramPivots) - 1; i > 0; i-- {
		p1 := histogramPivots[i]
		p2 := histogramPivots[i-1]
		diff := p1 - p2

		if diff > -minDiff || diff > minDiff {
			continue
		}

		// negative value = MACD top divergence
		if diff < -minDiff {
			logrus.Infof("MACD TOP DIVERGENCE DETECTED: diff %f", diff)
			topDivergence = true
		} else {
			topDivergence = false
		}
		return topDivergence
	}

	return false
}
