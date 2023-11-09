package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"

	"github.com/c9s/bbgo/pkg/types"
)

// Vortex Indicator. It provides two oscillators that capture positive and
// negative trend movement. A bullish signal triggers when the positive
// trend indicator crosses above the negative trend indicator or a key
// level. A bearish signal triggers when the negative trend indicator
// crosses above the positive trend indicator or a key level.
//
// +VM = Abs(Current High - Prior Low)
// -VM = Abd(Current Low - Prior High)
//
// +VM14 = 14-Period Sum of +VM
// -VM14 = 14-Period Sum of -VM
//
// TR = Max((High[i]-Low[i]), Abs(High[i]-Closing[i-1]), Abs(Low[i]-Closing[i-1]))
// TR14 = 14-Period Sum of TR
//
// +VI14 = +VM14 / TR14
// -VI14 = -VM14 / TR14
//
// Based on https://school.stockcharts.com/doku.php?id=technical_indicators:vortex_indicator
type VortexStream struct {
	plusVm, minusVm, tr          floats.Slice
	PlusVi, MinusVi              *types.Float64Series
	plusVmSum, minusVmSum, trSum float64
	window                       int
}

func Vortex(source KLineSubscription) *VortexStream {
	var (
		low     = LowPrices(source)
		high    = HighPrices(source)
		closing = ClosePrices(source)
		window  = 14
		s       = &VortexStream{
			PlusVi:  types.NewFloat64Series(),
			MinusVi: types.NewFloat64Series(),
			plusVm:  make([]float64, window),
			minusVm: make([]float64, window),
			tr:      make([]float64, window),
			window:  window,
		}
	)

	source.AddSubscriber(func(v types.KLine) {
		var (
			i = source.Length()
			j = i % s.window
		)
		if i == 1 {
			s.PlusVi.PushAndEmit(0)
			s.MinusVi.PushAndEmit(0)
			return
		}
		s.plusVmSum -= s.plusVm[j]
		s.plusVm[j] = math.Abs(high.Last(0) - low.Last(1))
		s.plusVmSum += s.plusVm[j]

		s.minusVmSum -= s.minusVm[j]
		s.minusVm[j] = math.Abs(low.Last(0) - high.Last(1))
		s.minusVmSum += s.minusVm[j]

		var (
			highLow         = high.Last(0) - low.Last(0)
			highPrevClosing = math.Abs(high.Last(0) - closing.Last(1))
			lowPrevClosing  = math.Abs(low.Last(0) - closing.Last(1))
		)

		s.trSum -= s.tr[j]
		s.tr[j] = math.Max(highLow, math.Max(highPrevClosing, lowPrevClosing))
		s.trSum += s.tr[j]

		s.PlusVi.PushAndEmit(s.plusVmSum / s.trSum)
		s.MinusVi.PushAndEmit(s.minusVmSum / s.trSum)
	})
	return s
}

func (s *VortexStream) Truncate() {
	s.PlusVi.Slice = s.PlusVi.Slice.Truncate(MaxNumOfMA)
	s.MinusVi.Slice = s.MinusVi.Slice.Truncate(MaxNumOfMA)
}
