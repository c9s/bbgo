package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

type DxStream struct {
	*types.Float64Series
	window   int
	tr       *TRStream
	sTr      *smooth
	sPlusDm  *smooth
	sMinusDm *smooth
}

func DirectionalIndex(source KLineSubscription, window int) *DxStream {
	high := HighPrices(source)
	low := LowPrices(source)
	s := &DxStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		tr:            TR2(source),
		sTr:           WilderSmoothing(window),
		sPlusDm:       WilderSmoothing(window),
		sMinusDm:      WilderSmoothing(window),
	}
	source.AddSubscriber(func(kLine types.KLine) {
		if source.Length() == 1 {
			s.PushAndEmit(0)
			return
		}
		tr := s.tr.Last(0)
		plusDm := high.Last(0) - high.Last(1)
		minusDm := low.Last(1) - low.Last(0)
		if minusDm > plusDm || plusDm < 0 {
			plusDm = 0
		}
		if plusDm > minusDm || minusDm < 0 {
			minusDm = 0
		}

		str := s.sTr.update(tr)
		spDm := s.sPlusDm.update(plusDm)
		smDm := s.sMinusDm.update(minusDm)
		if source.Length() < s.window {
			s.PushAndEmit(0)
			return
		}
		dx := .0
		if !almostZero(str) {
			// NOTE: when tr is non-zero, str * 100.0 seems cancelled out in the formula?
			pDi := spDm /// str * 100.0
			mDi := smDm /// str * 100.0
			sumDi := pDi + mDi
			if !almostZero(sumDi) {
				dx = math.Abs(pDi-mDi) / sumDi * 100
			}
		}

		s.PushAndEmit(dx)
	})

	return s
}

type smooth struct {
	window int
	k      float64
	sz     int
	sm     float64
}

func WilderSmoothing(window int) *smooth {
	return &smooth{
		window: window,
		k:      float64(window-1) / float64(window),
		sz:     0,
		sm:     0,
	}
}

func (s *smooth) update(v float64) float64 {
	s.sz++
	if s.sz < s.window {
		s.sm += v
		return 0
	} else {
		s.sm = s.sm*s.k + v
	}

	return s.sm
}

func almostZero(v float64) bool {
	return v > -0.00000000000001 && v < 0.00000000000001
}
