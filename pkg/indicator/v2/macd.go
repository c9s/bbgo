package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type MACDStream struct {
	*SubtractStream

	shortWindow, longWindow, signalWindow int

	FastEWMA, SlowEWMA, Signal *EWMAStream
	Histogram                  *SubtractStream
}

func MACD2(source types.Float64Source, shortWindow, longWindow, signalWindow int) *MACDStream {
	// bind and calculate these first
	fastEWMA := EWMA2(source, shortWindow)
	slowEWMA := EWMA2(source, longWindow)
	macd := Subtract(fastEWMA, slowEWMA)
	signal := EWMA2(macd, signalWindow)
	histogram := Subtract(macd, signal)
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
