package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ADXStream struct {
	*types.Float64Series
	Plus, Minus *types.Float64Series

	window            int
	prevHigh, prevLow fixedpoint.Value
}

func ADX(source KLineSubscription, window int) *ADXStream {
	var (
		atr  = ATR2(source, window)
		dmp  = types.NewFloat64Series()
		dmm  = types.NewFloat64Series()
		adx  = types.NewFloat64Series()
		sdmp = RMA2(dmp, window, true)
		sdmm = RMA2(dmm, window, true)
		sadx = RMA2(adx, window, true)
		s    = &ADXStream{
			window:        window,
			Plus:          types.NewFloat64Series(),
			Minus:         types.NewFloat64Series(),
			Float64Series: types.NewFloat64Series(),
			prevHigh:      fixedpoint.Zero,
			prevLow:       fixedpoint.Zero,
		}
	)

	source.AddSubscriber(func(k types.KLine) {
		up, down := k.High.Sub(s.prevHigh), -k.Low.Sub(s.prevLow)
		if up.Compare(down) > 0 && up > 0 {
			dmp.PushAndEmit(up.Float64())
		} else {
			dmp.PushAndEmit(0.0)
		}

		if down.Compare(up) > 0 && down > 0 {
			dmm.PushAndEmit(down.Float64())
		} else {
			dmm.PushAndEmit(0.0)
		}

		s.Plus.PushAndEmit(sdmp.Last(0) / atr.Last(0) * 100)
		s.Minus.PushAndEmit(sdmm.Last(0) / atr.Last(0) * 100)

		sum := s.Plus.Last(0) + s.Minus.Last(0)
		if sum == 0 {
			sum = 1
		}
		adx.PushAndEmit(math.Abs(s.Plus.Last(0)-s.Minus.Last(0)) / sum)

		s.PushAndEmit(sadx.Last(0) * 100)
		s.prevHigh, s.prevLow = k.High, k.Low
		s.Truncate()
	})
	return s
}

func (s *ADXStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfRMA)
}
