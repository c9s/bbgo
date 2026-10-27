package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// TSIStream is the True Strength Index
//
// Refer: True Strength Index
// Refer URL: https://www.investopedia.com/terms/t/tsi.asp
//
// The True Strength Index (TSI) is a momentum oscillator based on a double smoothing of price changes:
//
//	pc = x - x[1]
//	PCS = EMA(pc, slowWindow), PCDS = EMA(PCS, fastWindow)
//	APCS = EMA(|pc|, slowWindow), APCDS = EMA(APCS, fastWindow)
//	TSI = PCDS / APCDS * 100
//
// The first input is only used as the previous value, so no value is emitted for it.
//
// Note: the inner moving averages are EWMA2 streams, which treat a last value of exactly 0 as "not initialized"
// and re-seed with the next input. With a leading run of zero price changes this differs slightly from the legacy
// implementation during warm-up; the difference decays geometrically.
type TSIStream struct {
	*types.Float64Series

	fastWindow, slowWindow int

	prevValue   float64
	initialized bool

	Pcs, Pcds, Apcs, Apcds *EWMAStream
}

// TSI creates the True Strength Index stream.
// When fastWindow or slowWindow is 0, the default 13 and 25 are used respectively.
func TSI(source types.Float64Source, fastWindow, slowWindow int) *TSIStream {
	if fastWindow == 0 {
		fastWindow = 13
	}
	if slowWindow == 0 {
		slowWindow = 25
	}

	s := &TSIStream{
		Float64Series: types.NewFloat64Series(),
		fastWindow:    fastWindow,
		slowWindow:    slowWindow,
		// the inner EWMA streams are driven by calculate
		Pcs:   EWMA2(nil, slowWindow),
		Pcds:  EWMA2(nil, fastWindow),
		Apcs:  EWMA2(nil, slowWindow),
		Apcds: EWMA2(nil, fastWindow),
	}

	if source != nil {
		s.Subscribe(source, func(v float64) {
			if y, ok := s.calculate(v); ok {
				s.PushAndEmit(y)
				s.Truncate()
			}
		})
	}
	return s
}

func (s *TSIStream) calculate(v float64) (float64, bool) {
	if !s.initialized {
		s.initialized = true
		s.prevValue = v
		return 0, false
	}

	pc := v - s.prevValue
	s.prevValue = v

	update := func(e *EWMAStream, x float64) float64 {
		e.PushAndEmit(e.Calculate(x))
		e.Truncate()
		return e.Last(0)
	}

	pcs := update(s.Pcs, pc)
	apcs := update(s.Apcs, math.Abs(pc))
	pcds := update(s.Pcds, pcs)
	apcds := update(s.Apcds, apcs)
	return (pcds / apcds) * 100., true
}

func (s *TSIStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}
