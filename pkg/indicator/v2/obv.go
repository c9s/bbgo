package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

type OBVStream struct {
	*types.Float64Series
}

// The [Obv](https://pkg.go.dev/github.com/cinar/indicator#Obv) function calculates a technical
// trading momentum indicator that uses volume flow to predict changes in stock price.
func OBV(source KLineSubscription) *OBVStream {
	s := &OBVStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(v types.KLine) {
		var obv = .0

		if source.Length() > 1 {
			prev := source.Last(1)
			obv = s.Slice.Last(0)

			if v.Close.Float64() > prev.Close.Float64() {
				obv += v.Volume.Float64()
			} else if v.Close.Float64() < prev.Close.Float64() {
				obv -= v.Volume.Float64()
			}
		}

		s.PushAndEmit(obv)
	})

	return s
}

func (s *OBVStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfMA)
}
