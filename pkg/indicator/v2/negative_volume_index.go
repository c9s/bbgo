package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// Starting value for NVI.
var NVI_STARTING_VALUE = fixedpoint.NewFromInt(1000)

// Negative Volume Index (NVI)
type NVIStream struct {
	*types.Float64Series
}

// The [NegativeVolumeIndex](https://pkg.go.dev/github.com/cinar/indicator#NegativeVolumeIndex)
// function calculates a cumulative indicator using the change in volume to decide when the smart money is active.
//
//			If Volume is greather than Previous Volume:
//	   			NVI = Previous NVI
//			Otherwise:
//				NVI = Previous NVI + (((Closing - Previous Closing) / Previous Closing) * Previous NVI)
func NegativeVolumeIndex(source KLineSubscription) *NVIStream {
	s := &NVIStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(v types.KLine) {
		var nvi = fixedpoint.Zero

		if s.Length() == 0 {
			nvi = NVI_STARTING_VALUE
		} else {
			var (
				prev    = source.Last(1)
				prevNVI = fixedpoint.NewFromFloat(s.Slice.Last(0))
			)
			if v.Volume > prev.Volume {
				nvi = prevNVI
			} else {
				inner := v.Close.Sub(prev.Close).Div(prev.Close)
				nvi = prevNVI.Add(inner.Mul(prevNVI))
			}
		}

		s.PushAndEmit(nvi.Float64())
	})

	return s
}

func (s *NVIStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfMA)
}
