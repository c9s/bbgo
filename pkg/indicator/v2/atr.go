package indicatorv2

type ATRStream struct {
	// embedded struct
	*RMAStream
}

func ATR2(source KLineSubscription, window int) *ATRStream {
	tr := TR2(source)
	rma := RMA2(tr, window, true)
	return &ATRStream{RMAStream: rma}
}
