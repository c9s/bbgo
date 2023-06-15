package indicator

type ATRPStream struct {
	*Float64Series
}

func ATRP2(source KLineSubscription, window int) *ATRPStream {
	s := &ATRPStream{
		Float64Series: NewFloat64Series(),
	}
	tr := TR2(source)
	atr := RMA2(tr, window, true)
	atr.OnUpdate(func(x float64) {
		// x is the last rma
		k := source.Last(0)
		cloze := k.Close.Float64()
		atrp := x / cloze
		s.PushAndEmit(atrp)
	})
	return s
}
