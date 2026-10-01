package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Chaikin Money Flow (CMF)
// - https://www.fidelity.com/learning-center/trading-investing/technical-analysis/technical-indicator-guide/cmf
// - https://www.investopedia.com/terms/c/chaikinmoneyflow.asp
//
// Money Flow Multiplier = ((Close - Low) - (High - Close)) / (High - Low)
// Money Flow Volume = Money Flow Multiplier * Volume
// CMF = Sum(Money Flow Volume, Window) / Sum(Volume, Window)
//
// A zero-range candle (High == Low) contributes zero money flow volume.
// No value is emitted before the window is filled.
type CMFStream struct {
	// embedded struct
	*types.Float64Series

	window int

	mfvQueue *types.Queue
	volQueue *types.Queue
}

func CMF2(source KLineSubscription, window int) *CMFStream {
	checkWindow(window)

	s := &CMFStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		mfvQueue:      types.NewQueue(window),
		volQueue:      types.NewQueue(window),
	}

	source.AddSubscriber(func(k types.KLine) {
		s.calculateAndPush(k.High.Float64(), k.Low.Float64(), k.Close.Float64(), k.Volume.Float64())
	})
	return s
}

func (s *CMFStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}

func (s *CMFStream) calculateAndPush(high, low, closePrice, volume float64) {
	var mfv float64
	if high > low {
		mfm := ((closePrice - low) - (high - closePrice)) / (high - low)
		mfv = mfm * volume
	}

	s.mfvQueue.Update(mfv)
	s.volQueue.Update(volume)

	if s.mfvQueue.Length() < s.window {
		return
	}

	var sumMFV, sumVol float64
	for i := 0; i < s.window; i++ {
		sumMFV += s.mfvQueue.Last(i)
		sumVol += s.volQueue.Last(i)
	}

	var cmf float64
	if sumVol != 0 {
		cmf = sumMFV / sumVol
	}

	s.PushAndEmit(cmf)
}
