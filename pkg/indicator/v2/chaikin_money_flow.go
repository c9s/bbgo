package indicatorv2

import (
	"gonum.org/v1/gonum/floats"

	bbfloat "github.com/c9s/bbgo/pkg/datatype/floats"

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
func ChaikinMoneyFlow(source KLineSubscription, window int) *ChaikinMoneyFlowStream {
	var (
		closing = ClosePrices(source)
		low     = LowPrices(source)
		high    = HighPrices(source)
		volume  = Volumes(source)
		cl      = Subtract(closing, low)
		hc      = Subtract(high, closing)
		flow    = Subtract(cl, hc)
		hl      = Subtract(high, low)
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

func ChaikinMoneyFlowDefault(source KLineSubscription) *ChaikinMoneyFlowStream {
	return ChaikinMoneyFlow(source, 20)
}
