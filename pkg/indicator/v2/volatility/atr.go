package volatility

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/indicator/v2/trend"
)

type ATRStream struct {
	// embedded struct
	*trend.RMAStream
}

func ATR2(source v2.KLineSubscription, window int) *ATRStream {
	tr := TR2(source)
	rma := trend.RMA2(tr, window, true)
	return &ATRStream{RMAStream: rma}
}
