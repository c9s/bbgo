package indicator

type ATRStream struct {
	// embedded struct
	*RMAStream
}

func ATR2(source KLineSubscription, window int) *ATRStream {
	s := &ATRStream{}
	tr := TR2(source)
	s.RMAStream = RMA2(tr, window, true)
	return s
}
