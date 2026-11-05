package supertrend

import (
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// LinReg is the Linear Regression baseline config, it is used as the trend confirmation.
// The slopes of the linear regression baseline are calculated on the kline close prices of the interval
// by indicatorv2.LinReg (0 before the window is filled).
type LinReg struct {
	types.IntervalWindow

	stream *indicatorv2.LinRegStream
}

// Bind creates the linear regression stream on the close prices of the given kline stream.
func (lr *LinReg) Bind(closePrices types.Float64Source) {
	lr.stream = indicatorv2.LinReg(closePrices, lr.Window)
}

// Last slope of linear regression baseline
func (lr *LinReg) Last(i int) float64 {
	if lr.stream == nil {
		return 0
	}

	return lr.stream.Last(i)
}

// Length of the slope values
func (lr *LinReg) Length() int {
	if lr.stream == nil {
		return 0
	}

	return lr.stream.Length()
}

// GetSignal get linear regression signal
func (lr *LinReg) GetSignal() types.Direction {
	var lrSignal types.Direction = types.DirectionNone

	switch {
	case lr.Last(0) > 0:
		lrSignal = types.DirectionUp
	case lr.Last(0) < 0:
		lrSignal = types.DirectionDown
	}

	return lrSignal
}
