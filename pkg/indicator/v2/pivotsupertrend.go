package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// PivotSupertrendStream is based on "Pivot Point Supertrend by LonesomeTheBlue" from tradingview.
//
// Instead of the (high + low) / 2 center line of the classic supertrend, the center
// line is a weighted average of the most recent pivot points:
//
//	center = (center * 2 + lastPivot) / 3
//	up     = center - ATR * multiplier
//	down   = center + ATR * multiplier
//
// The up/down bands are ratcheted like the classic supertrend and the trend flips
// when the close price crosses the previous band. The emitted value is the trend
// line: the support line in an uptrend, the resistance line in a downtrend.
// No value is emitted until the first pivot high or pivot low is found.
type PivotSupertrendStream struct {
	*types.Float64Series

	// SupportLine is the support line in an uptrend (green)
	SupportLine *types.Float64Series

	// ResistanceLine is the resistance line in a downtrend (red)
	ResistanceLine *types.Float64Series

	ATRMultiplier float64

	atr       *ATRStream
	pivotLow  *PivotLowStream
	pivotHigh *PivotHighStream

	closePrice             float64
	previousClosePrice     float64
	uptrendPrice           float64
	previousUptrendPrice   float64
	downtrendPrice         float64
	previousDowntrendPrice float64

	lastPp            float64
	src               float64 // center
	previousPivotHigh float64 // the pivot high value before the current kline
	previousPivotLow  float64 // the pivot low value before the current kline

	trend         types.Direction
	previousTrend types.Direction
	tradeSignal   types.Direction
}

// PivotSupertrend creates a pivot point supertrend stream.
// window is the ATR window, pivotWindow is the left/right window of the pivot
// high/low detection, atrMultiplier is the ATR band multiplier.
func PivotSupertrend(source KLineSubscription, window, pivotWindow int, atrMultiplier float64) *PivotSupertrendStream {
	checkWindow(window)
	checkWindow(pivotWindow)

	s := &PivotSupertrendStream{
		Float64Series:  types.NewFloat64Series(),
		SupportLine:    types.NewFloat64Series(),
		ResistanceLine: types.NewFloat64Series(),
		ATRMultiplier:  atrMultiplier,
		atr:            ATR2(source, window),
		pivotLow:       PivotLow(LowPrices(source), pivotWindow),
		pivotHigh:      PivotHigh(HighPrices(source), pivotWindow),
		trend:          types.DirectionUp,
	}

	source.AddSubscriber(func(k types.KLine) {
		s.update(k.Close.Float64())
	})

	return s
}

// update is called after the pivot high/low and ATR streams have been updated
// with the current kline.
func (s *PivotSupertrendStream) update(closePrice float64) {
	previousPivotLow := s.previousPivotLow
	previousPivotHigh := s.previousPivotHigh
	s.previousPivotLow = s.pivotLow.Last(0)
	s.previousPivotHigh = s.pivotHigh.Last(0)

	atr := s.atr.Last(0)

	// Update last prices
	s.previousUptrendPrice = s.uptrendPrice
	s.previousDowntrendPrice = s.downtrendPrice
	s.previousClosePrice = s.closePrice
	s.previousTrend = s.trend

	s.closePrice = closePrice

	// Initialize lastPp as soon as pivots are made
	if s.lastPp == 0 || math.IsNaN(s.lastPp) {
		if s.pivotHigh.Length() > 0 {
			s.lastPp = s.pivotHigh.Last(0)
		} else if s.pivotLow.Length() > 0 {
			s.lastPp = s.pivotLow.Last(0)
		} else {
			s.lastPp = math.NaN()
			return
		}
	}

	// Set lastPp to the latest pivotPoint (only changed when new pivot is found)
	if s.pivotHigh.Last(0) != previousPivotHigh {
		s.lastPp = s.pivotHigh.Last(0)
	} else if s.pivotLow.Last(0) != previousPivotLow {
		s.lastPp = s.pivotLow.Last(0)
	}

	// calculate the Center line using pivot points
	if s.src == 0 || math.IsNaN(s.src) {
		s.src = s.lastPp
	} else {
		// weighted calculation
		s.src = (s.src*2 + s.lastPp) / 3
	}

	// Update uptrend
	s.uptrendPrice = s.src - atr*s.ATRMultiplier
	if s.previousClosePrice > s.previousUptrendPrice {
		s.uptrendPrice = math.Max(s.uptrendPrice, s.previousUptrendPrice)
	}

	// Update downtrend
	s.downtrendPrice = s.src + atr*s.ATRMultiplier
	if s.previousClosePrice < s.previousDowntrendPrice {
		s.downtrendPrice = math.Min(s.downtrendPrice, s.previousDowntrendPrice)
	}

	// Update trend
	if s.previousTrend == types.DirectionUp && s.closePrice < s.previousUptrendPrice {
		s.trend = types.DirectionDown
	} else if s.previousTrend == types.DirectionDown && s.closePrice > s.previousDowntrendPrice {
		s.trend = types.DirectionUp
	} else {
		s.trend = s.previousTrend
	}

	// Update signal
	if atr <= 0 {
		s.tradeSignal = types.DirectionNone
	} else if s.trend == types.DirectionUp && s.previousTrend == types.DirectionDown {
		s.tradeSignal = types.DirectionUp
	} else if s.trend == types.DirectionDown && s.previousTrend == types.DirectionUp {
		s.tradeSignal = types.DirectionDown
	} else {
		s.tradeSignal = types.DirectionNone
	}

	// Save the trend lines
	s.SupportLine.PushAndEmit(s.uptrendPrice)
	s.ResistanceLine.PushAndEmit(s.downtrendPrice)
	s.SupportLine.Slice = generalTruncate(s.SupportLine.Slice)
	s.ResistanceLine.Slice = generalTruncate(s.ResistanceLine.Slice)

	// Update trend price
	if s.trend == types.DirectionDown {
		s.PushAndEmit(s.downtrendPrice)
	} else {
		s.PushAndEmit(s.uptrendPrice)
	}
	s.Slice = generalTruncate(s.Slice)
}

// GetSignal returns signal (Down, None or Up)
func (s *PivotSupertrendStream) GetSignal() types.Direction {
	return s.tradeSignal
}

// Direction returns the current trend
func (s *PivotSupertrendStream) Direction() types.Direction {
	return s.trend
}

// LastSupertrendSupport returns the current supertrend support value
func (s *PivotSupertrendStream) LastSupertrendSupport() float64 {
	return s.SupportLine.Last(0)
}

// LastSupertrendResistance returns the current supertrend resistance value
func (s *PivotSupertrendStream) LastSupertrendResistance() float64 {
	return s.ResistanceLine.Last(0)
}
