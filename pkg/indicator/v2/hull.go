package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// HULLStream is the Hull Moving Average
//
// Refer: Hull Moving Average
// Refer URL: https://fidelity.com/learning-center/trading-investing/technical-analysis/technical-indicator-guide/hull-moving-average
//
// The Hull Moving Average (HMA) is a technical analysis indicator that uses a weighted moving average to reduce the lag in simple moving averages.
// It was developed by Alan Hull, who sought to create a moving average that was both fast and smooth. The HMA is calculated by first taking
// the weighted moving average of the input data using a weighting factor of W, where W is the square root of the length of the moving average.
// The result is then double-smoothed by taking the weighted moving average of this result using a weighting factor of W/2. This final average
// forms the HMA line, which can be used to make predictions about future price movements.
//
// Note: like the legacy implementation, exponential moving averages are used in place of weighted moving averages:
//
//	HULL = EMA(2 * EMA(x, window/2) - EMA(x, window), int(sqrt(window)))
type HULLStream struct {
	*types.Float64Series

	window int

	// MA1 is EMA(x, window/2), MA2 is EMA(x, window)
	MA1, MA2 *EWMAStream

	result *EWMAStream
}

func HULL(source types.Float64Source, window int) *HULLStream {
	s := &HULLStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		// the inner EWMA streams are driven by Calculate
		MA1:    EWMA2(nil, window/2),
		MA2:    EWMA2(nil, window),
		result: EWMA2(nil, int(math.Sqrt(float64(window)))),
	}
	s.Bind(source, s)
	return s
}

func (s *HULLStream) Calculate(v float64) float64 {
	s.MA1.PushAndEmit(s.MA1.Calculate(v))
	s.MA1.Truncate()

	s.MA2.PushAndEmit(s.MA2.Calculate(v))
	s.MA2.Truncate()

	s.result.PushAndEmit(s.result.Calculate(2*s.MA1.Last(0) - s.MA2.Last(0)))
	s.result.Truncate()
	return s.result.Last(0)
}

func (s *HULLStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}
