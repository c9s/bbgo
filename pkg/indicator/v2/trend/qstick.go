package trend

import (
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type QstickStream struct {
	*types.Float64Series
	sma *SMAStream
}

// The Qstick function calculates the ratio of recent up and down bars.
//
// QS = Sma(Closing - Opening)
func Qstick(source v2.KLineSubscription, window int) *QstickStream {
	var (
		s = &QstickStream{
			Float64Series: types.NewFloat64Series(),
			sma:           SMA(v2.CloseSubOpen(source), window),
		}
	)
	source.AddSubscriber(func(v types.KLine) {
		s.PushAndEmit(s.sma.Last(0))
	})

	return s
}
