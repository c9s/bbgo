package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// ADStream implements the accumulation/distribution indicator
//
// Accumulation/Distribution Indicator (A/D)
// - https://www.investopedia.com/terms/a/accumulationdistribution.asp
//
// A/D is a running total of the money flow volume of each kline:
//
//	MFV = ((close - low) - (high - close)) / (high - low) * volume
//	AD  = AD[-1] + MFV
//
// When high == low the money flow volume is treated as zero.
type ADStream struct {
	*types.Float64Series
}

func AD(source KLineSubscription) *ADStream {
	s := &ADStream{
		Float64Series: types.NewFloat64Series(),
	}

	source.AddSubscriber(func(k types.KLine) {
		s.PushAndEmit(s.calculate(k.High.Float64(), k.Low.Float64(), k.Close.Float64(), k.Volume.Float64()))
		s.Truncate()
	})

	return s
}

func (s *ADStream) calculate(high, low, cloze, volume float64) float64 {
	moneyFlowVolume := 0.0
	if high != low {
		moneyFlowVolume = ((2*cloze - high - low) / (high - low)) * volume
	}

	return s.Last(0) + moneyFlowVolume
}

func (s *ADStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}
