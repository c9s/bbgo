package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

type TrendLineStream struct {
	*types.Float64Series
}

// NewTrendlineIndicator returns an indicator whose output is the slope of the trend
// line given by the values in the window.
func TrendLine(source KLineSubscription, window int) *TrendLineStream {
	var (
		closing = ClosePrices(source)
		s       = &TrendLineStream{
			Float64Series: types.NewFloat64Series(),
		}
	)
	source.AddSubscriber(func(v types.KLine) {
		var index = source.Length()
		if index < window {
			s.PushAndEmit(0.0)
		}
		var (
			values = closing.Slice.Tail(window)
			ab     = sumXy(values)*float64(window) - sumX(values)*sumY(values)
			cd     = sumX2(values)*float64(window) - math.Pow(sumX(values), 2)
			trend  = ab / cd
		)
		s.PushAndEmit(trend)
	})
	return s
}

func (s *TrendLineStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfMA)
}

func sumX(s []float64) (sum float64) {
	for i := range s {
		sum += float64(i)
	}
	return
}

func sumY(s []float64) (sum float64) {
	for _, d := range s {
		sum += d
	}
	return
}

func sumXy(s []float64) (sum float64) {
	for i, d := range s {
		sum += d * float64(i)
	}
	return
}

func sumX2(s []float64) (sum float64) {
	for i := range s {
		sum += math.Pow(float64(i), 2)
	}
	return
}
