package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// KalmanFilterStream implements a one-dimensional Kalman filter
//
// Refer: https://www.kalmanfilter.net/kalman1d.html
//
// The measurement uncertainty is the root mean square of the amplitudes of the last `window`
// measurements, scaled by (1 + additionalSmoothWindow).
type KalmanFilterStream struct {
	*types.Float64Series

	window                 int
	additionalSmoothWindow uint

	amp2         *types.Queue // measurement uncertainty
	k            float64      // Kalman gain
	measurements *types.Queue
}

func newKalmanFilterStream(window int, additionalSmoothWindow uint) *KalmanFilterStream {
	return &KalmanFilterStream{
		Float64Series:          types.NewFloat64Series(),
		window:                 window,
		additionalSmoothWindow: additionalSmoothWindow,
	}
}

// KalmanFilter creates a Kalman filter on the kline close price, using half of the
// kline range ((high - low) / 2) as the measurement amplitude.
func KalmanFilter(source KLineSubscription, window int, additionalSmoothWindow uint) *KalmanFilterStream {
	s := newKalmanFilterStream(window, additionalSmoothWindow)
	if source != nil {
		source.AddSubscriber(func(k types.KLine) {
			s.PushAndEmit(s.update(k.Close.Float64(), (k.High.Float64()-k.Low.Float64())/2))
			s.Truncate()
		})
	}
	return s
}

// KalmanFilterFromPrices creates a Kalman filter on a float64 source, using the absolute
// change between the current and the last measurement as the measurement amplitude.
func KalmanFilterFromPrices(source types.Float64Source, window int, additionalSmoothWindow uint) *KalmanFilterStream {
	s := newKalmanFilterStream(window, additionalSmoothWindow)
	s.Bind(source, s)
	return s
}

// Calculate updates the filter with the measurement v, using |v - last measurement| as the amplitude
func (s *KalmanFilterStream) Calculate(v float64) float64 {
	measureMove := v
	if s.measurements != nil {
		measureMove = v - s.measurements.Last(0)
	}
	return s.update(v, math.Abs(measureMove))
}

func (s *KalmanFilterStream) update(value, amp float64) float64 {
	if s.Length() == 0 {
		s.amp2 = types.NewQueue(s.window)
		s.amp2.Update(amp * amp)
		s.measurements = types.NewQueue(s.window)
		s.measurements.Update(value)
		return value
	}

	// measurement
	s.measurements.Update(value)
	s.amp2.Update(amp * amp)
	q := math.Sqrt(types.Mean(s.amp2)) * float64(1+s.additionalSmoothWindow)

	// update
	lastPredict := s.Slice.Last(0)
	curState := value + (value - lastPredict)
	estimated := lastPredict + s.k*(curState-lastPredict)

	// predict
	p := math.Abs(curState - estimated)
	s.k = p / (p + q)
	return estimated
}

func (s *KalmanFilterStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}
