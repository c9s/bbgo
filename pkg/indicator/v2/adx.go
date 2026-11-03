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
	initialized       bool
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
		if !s.initialized {
			s.initialized = true
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

		diPlus := 0.0
		diMinus := 0.0
		if atrVal := atr.Last(0); atrVal != 0 {
			k := 100.0 / atrVal
			diPlus = sdmp.Last(0) * k
			diMinus = sdmn.Last(0) * k
		}
		s.Plus.PushAndEmit(diPlus)
		s.Minus.PushAndEmit(diMinus)

		dx := 0.0
		if diSum := diPlus + diMinus; diSum != 0 {
			dx = 100.0 * math.Abs(diPlus-diMinus) / diSum
		}
		adx.PushAndEmit(dx)

		s.prevHigh, s.prevLow = k.High, k.Low
		s.Truncate()
	})
	return s
}

func (s *ADXStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfRMA)
	s.Plus.Slice = s.Plus.Slice.Truncate(MaxNumOfRMA)
	s.Minus.Slice = s.Minus.Slice.Truncate(MaxNumOfRMA)
}
