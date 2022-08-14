package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
	"math"
)

// Refer: https://www.kalmanfilter.net/kalman1d.html
// One-dimensional Kalman filter

//go:generate callbackgen -type KalmanFilter
type KalmanFilter struct {
	types.SeriesBase
	types.IntervalWindow
	AdditionalSmoothWindow uint
	amp2                   *types.Queue // measurement uncertainty
	k                      float64      // Kalman gain
	measurements           *types.Queue
	Values                 floats.Slice

	UpdateCallbacks []func(value float64)
}

func (inc *KalmanFilter) Update(value float64) {
	var measureMove = value
	if inc.measurements != nil {
		measureMove = value - inc.measurements.Last()
	}
	inc.update(value, math.Abs(measureMove))
}

func (inc *KalmanFilter) update(value, amp float64) {
	if len(inc.Values) == 0 {
		inc.amp2 = types.NewQueue(inc.Window)
		inc.amp2.Update(amp * amp)
		inc.measurements = types.NewQueue(inc.Window)
		inc.measurements.Update(value)
		inc.Values.Push(value)
		return
	}

	// measurement
	inc.measurements.Update(value)
	inc.amp2.Update(amp * amp)
	q := math.Sqrt(types.Mean(inc.amp2)) * float64(1+inc.AdditionalSmoothWindow)

	// update
	lastPredict := inc.Values.Last()
	curState := value + (value - lastPredict)
	estimated := lastPredict + inc.k*(curState-lastPredict)

	// predict
	inc.Values.Push(estimated)
	p := math.Abs(curState - estimated)
	inc.k = p / (p + q)
}

func (inc *KalmanFilter) Index(i int) float64 {
	if inc.Values == nil {
		return 0.0
	}
	return inc.Values.Index(i)
}

func (inc *KalmanFilter) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

func (inc *KalmanFilter) Last() float64 {
	if inc.Values == nil {
		return 0.0
	}
	return inc.Values.Last()
}

var _ types.SeriesExtend = &KalmanFilter{}

func (inc *KalmanFilter) PushK(k types.KLine) {
	inc.update(k.Close.Float64(), (k.High.Float64()-k.Low.Float64())/2)
}

func (inc *KalmanFilter) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.Values != nil {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last())
		return
	}
	for _, k := range allKLines {
		inc.PushK(k)
		inc.EmitUpdate(inc.Last())
	}
}

func (inc *KalmanFilter) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}
	inc.CalculateAndUpdate(window)
}

func (inc *KalmanFilter) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
