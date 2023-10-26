package trend

import (
	"math"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// Parabolic SAR. It is a popular technical indicator for identifying the trend
// and as a trailing stop.
//
// PSAR = PSAR[i - 1] - ((PSAR[i - 1] - EP) * AF)
//
// If the trend is Falling:
//   - PSAR is the maximum of PSAR or the previous two high values.
//   - If the current high is greather than or equals to PSAR, use EP.
//
// If the trend is Rising:
//   - PSAR is the minimum of PSAR or the previous two low values.
//   - If the current low is less than or equals to PSAR, use EP.
//
// If PSAR is greater than the closing, trend is falling, and the EP
// is set to the minimum of EP or the low.
//
// If PSAR is lower than or equals to the closing, trend is rising, and the EP
// is set to the maximum of EP or the high.
//
// If the trend is the same, and AF is less than 0.20, increment it by 0.02.
// If the trend is not the same, set AF to 0.02.
//
// Based on video https://www.youtube.com/watch?v=MuEpGBAH7pw&t=0s.

// Trend indicator.
type Trend int

const (
	Falling    Trend = -1
	Rising     Trend = 1
	psarAfStep       = 0.02
	psarAfMax        = 0.20
)

type PSARStream struct {
	*types.Float64Series
	Trend  []Trend
	af, ep float64
}

func ParabolicSar(source v2.KLineSubscription) *PSARStream {
	var (
		low     = v2.LowPrices(source)
		high    = v2.HighPrices(source)
		closing = v2.ClosePrices(source)
		s       = &PSARStream{
			Float64Series: types.NewFloat64Series(),
			Trend:         []Trend{},
		}
	)
	source.AddSubscriber(func(v types.KLine) {
		var (
			psar float64
			i    = source.Length() - 1
		)
		if i == 0 {
			s.Trend = append(s.Trend, Falling)
			psar = high.Last(0)
			s.af = psarAfStep
			s.ep = low.Last(0)
			s.PushAndEmit(psar)
			return
		}

		var prevPsar = s.Slice.Last(0)

		psar = prevPsar - ((prevPsar - s.ep) * s.af)

		if s.Trend[i-1] == Falling {
			psar = math.Max(psar, high.Last(1))
			if i > 1 {
				psar = math.Max(psar, high.Last(2))
			}
			if high.Last(0) >= psar {
				psar = s.ep
			}
		} else {
			psar = math.Min(psar, low.Last(1))
			if i > 1 {
				psar = math.Min(psar, low.Last(2))
			}
			if low.Last(0) <= psar {
				psar = s.ep
			}
		}

		var prevEp = s.ep

		if psar > closing.Last(0) {
			s.Trend = append(s.Trend, Falling)
			s.ep = math.Min(s.ep, low.Last(0))
		} else {
			s.Trend = append(s.Trend, Rising)
			s.ep = math.Max(s.ep, high.Last(0))
		}

		if s.Trend[i] != s.Trend[i-1] {
			s.af = psarAfStep
		} else if prevEp != s.ep && s.af < psarAfMax {
			s.af += psarAfStep
		}

		s.PushAndEmit(psar)
	})

	return s
}

func (s *PSARStream) Truncate() {
	s.Slice = s.Slice.Truncate(MaxNumOfMA)
}
