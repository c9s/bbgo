package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// RSIStream is the Relative Strength Index with Wilder's smoothing.
//
// Refer: https://school.stockcharts.com/doku.php?id=technical_indicators:relative_strength_index_rsi
//
// The first average gain and average loss are the simple means of the first `window` price changes,
// and then they are smoothed with avg = (avg * (window - 1) + change) / window.
//
// The first RSI value is emitted once `window` price changes (window + 1 prices) have been received.
type RSIStream struct {
	// embedded structs
	*types.Float64Series

	// config fields
	window int

	// private states
	started          bool
	prev             float64
	numChanges       int
	avgGain, avgLoss float64
}

func RSI2(source types.Float64Source, window int) *RSIStream {
	checkWindow(window)

	s := &RSIStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
	}

	if source != nil {
		s.Subscribe(source, func(v float64) {
			if rsi, ok := s.calculate(v); ok {
				s.PushAndEmit(rsi)
				s.Truncate()
			}
		})
	}

	return s
}

func (s *RSIStream) calculate(v float64) (float64, bool) {
	if !s.started {
		// the first price, no change yet
		s.started = true
		s.prev = v
		return 0, false
	}

	change := v - s.prev
	s.prev = v

	gain, loss := 0.0, 0.0
	if change > 0 {
		gain = change
	} else {
		loss = -change
	}

	n := float64(s.window)
	s.numChanges++

	switch {
	case s.numChanges < s.window:
		// accumulate the sums for the first simple means
		s.avgGain += gain
		s.avgLoss += loss
		return 0, false

	case s.numChanges == s.window:
		s.avgGain = (s.avgGain + gain) / n
		s.avgLoss = (s.avgLoss + loss) / n

	default:
		s.avgGain = (s.avgGain*(n-1) + gain) / n
		s.avgLoss = (s.avgLoss*(n-1) + loss) / n
	}

	if s.avgLoss == 0 {
		return 100, true
	}

	rs := s.avgGain / s.avgLoss
	return 100.0 - (100.0 / (1.0 + rs)), true
}

func (s *RSIStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
}
