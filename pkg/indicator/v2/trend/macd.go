package trend

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type MACDStream struct {
	*v2.SubtractStream

	shortWindow, longWindow, signalWindow int

	FastEWMA, SlowEWMA, Signal *EWMAStream
	Histogram                  *v2.SubtractStream
}

func MACD2(source types.Float64Source, shortWindow, longWindow, signalWindow int) *MACDStream {
	// bind and calculate these first
	fastEWMA := EWMA2(source, shortWindow)
	slowEWMA := EWMA2(source, longWindow)
	macd := v2.Subtract(fastEWMA, slowEWMA)
	signal := EWMA2(macd, signalWindow)
	histogram := v2.Subtract(macd, signal)
	return &MACDStream{
		SubtractStream: macd,
		shortWindow:    shortWindow,
		longWindow:     longWindow,
		signalWindow:   signalWindow,
		FastEWMA:       fastEWMA,
		SlowEWMA:       slowEWMA,
		Signal:         signal,
		Histogram:      histogram,
	}
}
