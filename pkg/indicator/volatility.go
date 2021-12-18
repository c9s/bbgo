package indicator

import (
	"fmt"
	"math"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfVOL = 5_000
const MaxNumOfVOLTruncateSize = 100

//var zeroTime time.Time

//go:generate callbackgen -type VOLATILITY
type VOLATILITY struct {
	types.IntervalWindow
	Values  types.Float64Slice
	EndTime time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *VOLATILITY) Last() float64 {
	if len(inc.Values) == 0 {
		return 0.0
	}
	return inc.Values[len(inc.Values)-1]
}

func (inc *VOLATILITY) calculateAndUpdate(klines []types.KLine) {
	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	volatility, err := calculateVOLATILITY(recentT, inc.Window, KLineClosePriceMapper)
	if err != nil {
		log.WithError(err).Error("can not calculate volatility")
		return
	}
	inc.Values.Push(volatility)

	if len(inc.Values) > MaxNumOfVOL {
		inc.Values = inc.Values[MaxNumOfVOLTruncateSize-1:]
	}

	inc.EndTime = klines[end].GetEndTime().Time()

	inc.EmitUpdate(volatility)
}

func (inc *VOLATILITY) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.calculateAndUpdate(window)
}

func (inc *VOLATILITY) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateVOLATILITY(klines []types.KLine, window int, priceF KLinePriceMapper) (float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
	}

	sum := 0.0
	for _, k := range klines {
		sum += priceF(k)
	}

	avg := sum / float64(window)
	sv := 0.0 // sum of variance

	for _, j := range klines {
		// The use of Pow math function func Pow(x, y float64) float64
		sv += math.Pow(priceF(j)-avg, 2)
	}
	// The use of Sqrt math function func Sqrt(x float64) float64
	sd := math.Sqrt(sv / float64(len(klines)))
	return sd, nil
}
