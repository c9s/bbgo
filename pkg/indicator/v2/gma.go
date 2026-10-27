package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// GMAStream is the Geometric Moving Average.
//
// The Geometric Moving Average (GMA) is a technical analysis indicator that uses the geometric mean of a set of data points instead of
// the simple arithmetic mean used by traditional moving averages. It is calculated by taking the nth root of the product of the last n
// data points, where n is the length of the moving average. The resulting average is then plotted on the price chart as a line, which can
// be used to make predictions about future price movements. Because the GMA gives more weight to recent data points, it is typically more
// responsive to changes in the underlying data than a simple moving average.
//
// Calculation:
//
//	GMA = exp(SMA(log(price), window))
//
// No value is emitted until window values have been collected.
type GMAStream struct {
	*types.Float64Series

	// SMA is the moving average of the log values (including the warm-up values)
	SMA *SMAStream

	logs   *types.Float64Series
	window int
	count  int
}

// GMA creates the geometric moving average stream
func GMA(source types.Float64Source, window int) *GMAStream {
	checkWindow(window)

	logs := types.NewFloat64Series()
	s := &GMAStream{
		Float64Series: types.NewFloat64Series(),
		SMA:           SMA(logs, window),
		logs:          logs,
		window:        window,
	}

	s.Subscribe(source, func(v float64) {
		s.logs.PushAndEmit(math.Log(v))
		s.count++
		if s.count >= s.window {
			s.PushAndEmit(math.Exp(s.SMA.Last(0)))
		}
		s.Truncate()
	})
	return s
}

func (s *GMAStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
	s.logs.Slice = generalTruncate(s.logs.Slice)
}
