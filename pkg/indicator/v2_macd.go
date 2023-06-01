package indicator

type MACDStream struct {
	*SubtractStream

	shortWindow, longWindow, signalWindow int

	fastEWMA, slowEWMA, signal *EWMAStream
	histogram                  *SubtractStream
}

func MACD2(source Float64Source, shortWindow, longWindow, signalWindow int) *MACDStream {
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
		fastEWMA:       fastEWMA,
		slowEWMA:       slowEWMA,
		signal:         signal,
		histogram:      histogram,
	}
}
