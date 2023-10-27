package volume

import (
	"gonum.org/v1/gonum/floats"

	bbfloat "github.com/c9s/bbgo/pkg/datatype/floats"
	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

type ChaikinMoneyFlowStream struct {
	*types.Float64Series
	moneyFlowVolume bbfloat.Slice
	window          int
}

// The Chaikin Money Flow (CMF) measures the amount of money flow volume
// over a given period.
//
// Money Flow Multiplier = ((Closing - Low) - (High - Closing)) / (High - Low)
// Money Flow Volume = Money Flow Multiplier * Volume
// Chaikin Money Flow = Sum(20, Money Flow Volume) / Sum(20, Volume)
func ChaikinMoneyFlow(source v2.KLineSubscription, window int) *ChaikinMoneyFlowStream {
	var (
		closing = v2.ClosePrices(source)
		low     = v2.LowPrices(source)
		high    = v2.HighPrices(source)
		volume  = v2.Volumes(source)
		cl      = v2.Subtract(closing, low)
		hc      = v2.Subtract(high, closing)
		flow    = v2.Subtract(cl, hc)
		hl      = v2.Subtract(high, low)
		s       = &ChaikinMoneyFlowStream{
			Float64Series: types.NewFloat64Series(),
			window:        window,
		}
	)
	source.AddSubscriber(func(v types.KLine) {
		var moneyFlowMultiplier = flow.Last(0) / hl.Last(0)
		s.moneyFlowVolume.Push(moneyFlowMultiplier * volume.Last(0))
		var (
			sumFlowVol = floats.Sum(s.moneyFlowVolume.Tail(s.window))
			sumVol     = floats.Sum(volume.Slice.Tail(s.window))
			cmf        = sumFlowVol / sumVol
		)

		s.PushAndEmit(cmf)
	})
	return s
}

func (s *ChaikinMoneyFlowStream) Truncate() {
	s.Slice = s.Slice.Truncate(5000)
}

func ChaikinMoneyFlowDefault(source v2.KLineSubscription) *ChaikinMoneyFlowStream {
	return ChaikinMoneyFlow(source, 20)
}
