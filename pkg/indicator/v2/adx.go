package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ADXStream struct {
	*RMAStream

	Plus, Minus *types.Float64Series

	window            int
	prevHigh, prevLow fixedpoint.Value
}

func ADX(source KLineSubscription, window int) *ADXStream {
	var (
		atr  = ATR2(source, window)
		dmp  = types.NewFloat64Series()
		dmn  = types.NewFloat64Series()
		adx  = types.NewFloat64Series()
		sdmp = RMA2(dmp, window, true)
		sdmn = RMA2(dmn, window, true)
		s    = &ADXStream{
			window:    window,
			Plus:      types.NewFloat64Series(),
			Minus:     types.NewFloat64Series(),
			prevHigh:  fixedpoint.Zero,
			prevLow:   fixedpoint.Zero,
			RMAStream: RMA2(adx, window, true),
		}
	)

	source.AddSubscriber(func(k types.KLine) {
		if s.prevHigh.IsZero() || s.prevLow.IsZero() {
			s.prevHigh, s.prevLow = k.High, k.Low
			return
		}

		up, dn := k.High.Sub(s.prevHigh), s.prevLow.Sub(k.Low)
		if up.Compare(dn) > 0 && up.Float64() > 0 {
			dmp.PushAndEmit(up.Float64())
		} else {
			dmp.PushAndEmit(0.0)
		}
		if dn.Compare(up) > 0 && dn.Float64() > 0 {
			dmn.PushAndEmit(dn.Float64())
		} else {
			dmn.PushAndEmit(0.0)
		}

		s.Plus.PushAndEmit(sdmp.Last(0) * 100 / atr.Last(0))
		s.Minus.PushAndEmit(sdmn.Last(0) * 100 / atr.Last(0))
		dx := math.Abs(s.Plus.Last(0)-s.Minus.Last(0)) / (s.Plus.Last(0) + s.Minus.Last(0))
		if !math.IsNaN(dx) {
			adx.PushAndEmit(dx * 100.0)
		} else {
			adx.PushAndEmit(0.0)
		}

		s.prevHigh, s.prevLow = k.High, k.Low
		s.Truncate()
	})
	return s
}

func (s *ADXStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfRMA)
}
