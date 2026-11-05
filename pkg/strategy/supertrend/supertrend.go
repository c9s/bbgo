package supertrend

import (
	"math"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// SupertrendStream is the supertrend indicator used by this strategy.
//
// It keeps the band and trend flip rules of the legacy indicator.Supertrend, which differ from
// indicatorv2.SuperTrend: the trend flips when the close price crosses the band of the previous kline
// (indicatorv2.SuperTrend compares against the band of the current kline), and the bands are computed from
// the first kline on (indicatorv2.SuperTrend skips the klines before the ATR is available).
//
// The values of the stream are the trend prices: the support line in an uptrend, and the resistance line
// in a downtrend.
type SupertrendStream struct {
	*types.Float64Series

	// AverageTrueRange is the ATR used for calculating the bands
	AverageTrueRange *indicatorv2.ATRStream

	// SupportLine is the support line (the lower band) in an uptrend
	SupportLine *types.Float64Series

	// ResistanceLine is the resistance line (the upper band) in a downtrend
	ResistanceLine *types.Float64Series

	multiplier float64

	closePrice             float64
	previousClosePrice     float64
	uptrendPrice           float64
	previousUptrendPrice   float64
	downtrendPrice         float64
	previousDowntrendPrice float64

	trend         types.Direction
	previousTrend types.Direction
	tradeSignal   types.Direction
}

// NewSupertrendStream creates the supertrend stream of the given kline source.
// The ATR is subscribed to the source before the supertrend itself, so the source must be a stream
// without history, or be wrapped with indicatorv2.WithHistory.
func NewSupertrendStream(source indicatorv2.KLineSubscription, window int, multiplier float64) *SupertrendStream {
	if window <= 0 {
		panic("window must be greater than 0")
	}

	s := &SupertrendStream{
		Float64Series:    types.NewFloat64Series(),
		AverageTrueRange: indicatorv2.ATR2(source, window),
		SupportLine:      types.NewFloat64Series(),
		ResistanceLine:   types.NewFloat64Series(),
		multiplier:       multiplier,
		// start with DirectionUp
		trend: types.DirectionUp,
	}

	source.AddSubscriber(func(k types.KLine) {
		s.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	})

	return s
}

// Update updates the supertrend with the high, low and close price of a kline.
// The ATR must have been updated with the same kline before.
func (s *SupertrendStream) Update(highPrice, lowPrice, closePrice float64) {
	atr := s.AverageTrueRange.Last(0)

	// update last prices
	s.previousUptrendPrice = s.uptrendPrice
	s.previousDowntrendPrice = s.downtrendPrice
	s.previousClosePrice = s.closePrice
	s.previousTrend = s.trend

	s.closePrice = closePrice

	src := (highPrice + lowPrice) / 2

	// update uptrend
	s.uptrendPrice = src - atr*s.multiplier
	if s.previousClosePrice > s.previousUptrendPrice {
		s.uptrendPrice = math.Max(s.uptrendPrice, s.previousUptrendPrice)
	}

	// update downtrend
	s.downtrendPrice = src + atr*s.multiplier
	if s.previousClosePrice < s.previousDowntrendPrice {
		s.downtrendPrice = math.Min(s.downtrendPrice, s.previousDowntrendPrice)
	}

	// update trend
	if s.previousTrend == types.DirectionUp && s.closePrice < s.previousUptrendPrice {
		s.trend = types.DirectionDown
	} else if s.previousTrend == types.DirectionDown && s.closePrice > s.previousDowntrendPrice {
		s.trend = types.DirectionUp
	} else {
		s.trend = s.previousTrend
	}

	// update signal
	if atr <= 0 {
		s.tradeSignal = types.DirectionNone
	} else if s.trend == types.DirectionUp && s.previousTrend == types.DirectionDown {
		s.tradeSignal = types.DirectionUp
	} else if s.trend == types.DirectionDown && s.previousTrend == types.DirectionUp {
		s.tradeSignal = types.DirectionDown
	} else {
		s.tradeSignal = types.DirectionNone
	}

	// save the trend lines
	s.SupportLine.Push(s.uptrendPrice)
	s.ResistanceLine.Push(s.downtrendPrice)

	if s.trend == types.DirectionDown {
		s.PushAndEmit(s.downtrendPrice)
	} else {
		s.PushAndEmit(s.uptrendPrice)
	}

	s.Truncate()
}

func (s *SupertrendStream) Truncate() {
	s.Slice = types.ShrinkSlice(s.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
	s.SupportLine.Slice = types.ShrinkSlice(s.SupportLine.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
	s.ResistanceLine.Slice = types.ShrinkSlice(s.ResistanceLine.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
}

// GetSignal returns the trend flip of the last kline: DirectionUp or DirectionDown if the trend just flipped,
// DirectionNone otherwise.
func (s *SupertrendStream) GetSignal() types.Direction {
	return s.tradeSignal
}

// Direction returns the current trend
func (s *SupertrendStream) Direction() types.Direction {
	return s.trend
}

// LastSupertrendSupport returns the current supertrend support
func (s *SupertrendStream) LastSupertrendSupport() float64 {
	return s.SupportLine.Last(0)
}

// LastSupertrendResistance returns the current supertrend resistance
func (s *SupertrendStream) LastSupertrendResistance() float64 {
	return s.ResistanceLine.Last(0)
}
