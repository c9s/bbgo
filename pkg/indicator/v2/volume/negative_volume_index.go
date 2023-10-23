package volume

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// Starting value for NVI.
const NVI_STARTING_VALUE = 1000

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
func NegativeVolumeIndex(source v2.KLineSubscription) *NVIStream {
	s := &NVIStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(v types.KLine) {
		var nvi = fixedpoint.Zero

		if s.Slice.Length() == 0 {
			nvi = NVI_STARTING_VALUE
		} else {
			closing := source.Last(0).Close
			prevClose := source.Last(1).Close
			lastNVI := fixedpoint.NewFromFloat(s.Slice.Last(0))
			if source.Last(1).Volume < source.Last(0).Volume {
				nvi = lastNVI
			} else {
				nvi += ((closing - prevClose) / prevClose) * lastNVI
			}
		}

		s.PushAndEmit(nvi.Float64())
	})

	return s
}
