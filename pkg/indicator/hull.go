package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Hull Moving Average
// Refer URL: https://fidelity.com/learning-center/trading-investing/technical-analysis/technical-indicator-guide/hull-moving-average
//
// The Hull Moving Average (HMA) is a technical analysis indicator that uses a weighted moving average to reduce the lag in simple moving averages.
// It was developed by Alan Hull, who sought to create a moving average that was both fast and smooth. The HMA is calculated by first taking
// the weighted moving average of the input data using a weighting factor of W, where W is the square root of the length of the moving average.
// The result is then double-smoothed by taking the weighted moving average of this result using a weighting factor of W/2. This final average
// forms the HMA line, which can be used to make predictions about future price movements.
//
//go:generate callbackgen -type HULL
type HULL struct {
	types.SeriesBase
	types.IntervalWindow
	ma1    *EWMA
	ma2    *EWMA
	result *EWMA

	updateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &HULL{}

func (inc *HULL) Update(value float64) {
	if inc.result == nil {
		inc.SeriesBase.Series = inc
		inc.ma1 = &EWMA{IntervalWindow: types.IntervalWindow{Interval: inc.Interval, Window: inc.Window / 2}}
		inc.ma2 = &EWMA{IntervalWindow: inc.IntervalWindow}
		inc.result = &EWMA{IntervalWindow: types.IntervalWindow{Interval: inc.Interval, Window: int(math.Sqrt(float64(inc.Window)))}}
	}
	inc.ma1.Update(value)
	inc.ma2.Update(value)
	inc.result.Update(2*inc.ma1.Last(0) - inc.ma2.Last(0))
}

func (inc *HULL) Last(i int) float64 {
	if inc.result == nil {
		return 0
	}
	return inc.result.Last(i)
}

func (inc *HULL) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *HULL) Length() int {
	if inc.result == nil {
		return 0
	}
	return inc.result.Length()
}

func (inc *HULL) PushK(k types.KLine) {
	if inc.ma1 != nil && inc.ma1.Length() > 0 && k.EndTime.Before(inc.ma1.EndTime) {
		return
	}

	inc.Update(k.Close.Float64())
	inc.EmitUpdate(inc.Last(0))
}
