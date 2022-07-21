package factorzoo

import (
	"fmt"
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"

	log "github.com/sirupsen/logrus"
)

var zeroTime time.Time

type KLineValueMapper func(k types.KLine) float64

//go:generate callbackgen -type AVD
type AVD struct {
	types.SeriesBase
	types.IntervalWindow

	// Values
	Values    types.Float64Slice
	LastValue float64

	EndTime time.Time

	UpdateCallbacks []func(val float64)
}

func (inc *AVD) Index(i int) float64 {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Index(i)
}

func (inc *AVD) Last() float64 {
	if inc.Values.Length() == 0 {
		return 0
	}
	return inc.Values.Last()
}

func (inc *AVD) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

//var _ types.SeriesExtend = &AVD{}

func (inc *AVD) Update(klines []types.KLine) {
	if inc.Values == nil {
		inc.SeriesBase.Series = inc
	}

	if len(klines) < inc.Window {
		return
	}

	var end = len(klines) - 1
	var lastKLine = klines[end]

	if inc.EndTime != zeroTime && lastKLine.GetEndTime().Before(inc.EndTime) {
		return
	}

	var recentT = klines[end-(inc.Window-1) : end+1]

	val, err := calculateCorrelation(recentT, inc.Window, KLineAmplitudeMapper, indicator.KLineVolumeMapper)
	if err != nil {
		log.WithError(err).Error("can not calculate")
		return
	}
	val *= -1
	inc.Values.Push(val)
	inc.LastValue = val

	if len(inc.Values) > indicator.MaxNumOfVOL {
		inc.Values = inc.Values[indicator.MaxNumOfVOLTruncateSize-1:]
	}

	inc.EndTime = klines[end].GetEndTime().Time()

	inc.EmitUpdate(val)

}

func (inc *AVD) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.Update(window)
}

func (inc *AVD) Bind(updater indicator.KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}

func calculateCorrelation(klines []types.KLine, window int, valA KLineValueMapper, valB KLineValueMapper) (float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
	}

	sumA, sumB, sumAB, squareSumA, squareSumB := 0., 0., 0., 0., 0.
	for _, k := range klines {
		// sum of elements of array A
		sumA += valA(k)
		// sum of elements of array B
		sumB += valB(k)

		// sum of A[i] * B[i].
		sumAB = sumAB + valA(k)*valB(k)

		// sum of square of array elements.
		squareSumA = squareSumA + valA(k)*valA(k)
		squareSumB = squareSumB + valB(k)*valB(k)
	}
	// calculating correlation coefficient.
	corr := (float64(window)*sumAB - sumA*sumB) /
		math.Sqrt((float64(window)*squareSumA-sumA*sumA)*(float64(window)*squareSumB-sumB*sumB))

	return corr, nil
}

func KLineAmplitudeMapper(k types.KLine) float64 {
	return k.High.Div(k.Low).Float64()
}
