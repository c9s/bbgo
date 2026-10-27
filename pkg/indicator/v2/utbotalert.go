package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// UtBotAlertStream is based on "UT Bot Alerts by QuantNomad" from tradingview.
//
// It keeps an ATR trailing stop (ATR * keyValue away from the close price) and
// emits a signal for every kline: types.DirectionUp (buy) when the close crosses
// above the trailing stop, types.DirectionDown (sell) when the close crosses
// below it, and types.DirectionNone otherwise. The emitted float64 values are
// the types.Direction values (1, -1 or 0).
type UtBotAlertStream struct {
	*types.Float64Series

	// KeyValue is the ATR multiplier
	KeyValue float64

	// TrailingStop is the ATR trailing stop line (xATRTrailingStop)
	TrailingStop *types.Float64Series

	atr *ATRStream

	pos         types.Direction // NB: This is currently not in use (kept in case of expanding as it is in the tradingview version)
	previousPos types.Direction // NB: This is currently not in use (kept in case of expanding as it is in the tradingview version)

	previousClosePrice float64
}

// UtBotAlert creates a UT Bot Alert stream with the given ATR window and
// ATR multiplier (keyValue).
func UtBotAlert(source KLineSubscription, window int, keyValue float64) *UtBotAlertStream {
	checkWindow(window)

	s := &UtBotAlertStream{
		Float64Series: types.NewFloat64Series(),
		TrailingStop:  types.NewFloat64Series(),
		KeyValue:      keyValue,
		atr:           ATR2(source, window),
	}

	source.AddSubscriber(func(k types.KLine) {
		s.update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	})

	return s
}

func (s *UtBotAlertStream) update(highPrice, lowPrice, closePrice float64) {
	nLoss := s.atr.Last(0) * s.KeyValue

	// xATRTrailingStop
	// NB: the comparisons below use Last(1) of the trailing stop *before* the
	// new value is pushed, kept identical to the legacy indicator.
	ts := &s.TrailingStop.Slice
	if ts.Length() == 0 {
		// For first run
		ts.Update(0)
	} else if closePrice > ts.Last(1) && s.previousClosePrice > ts.Last(1) {
		ts.Update(math.Max(ts.Last(1), closePrice-nLoss))
	} else if closePrice < ts.Last(1) && s.previousClosePrice < ts.Last(1) {
		ts.Update(math.Min(ts.Last(1), closePrice+nLoss))
	} else if closePrice > ts.Last(1) {
		ts.Update(closePrice - nLoss)
	} else {
		ts.Update(closePrice + nLoss)
	}

	// pos
	if s.previousClosePrice < ts.Last(1) && closePrice > ts.Last(1) {
		s.pos = types.DirectionUp
	} else if s.previousClosePrice > ts.Last(1) && closePrice < ts.Last(1) {
		s.pos = types.DirectionDown
	} else {
		s.pos = s.previousPos
	}

	above := closePrice > ts.Last(0) && s.previousClosePrice < ts.Last(1)
	below := closePrice < ts.Last(0) && s.previousClosePrice > ts.Last(1)

	buy := closePrice > ts.Last(0) && above  // buy
	sell := closePrice < ts.Last(0) && below // sell

	// Update last prices
	s.previousClosePrice = closePrice
	s.previousPos = s.pos

	signal := types.DirectionNone
	if buy {
		signal = types.DirectionUp
	} else if sell {
		signal = types.DirectionDown
	}

	s.TrailingStop.EmitUpdate(ts.Last(0))
	s.TrailingStop.Slice = generalTruncate(s.TrailingStop.Slice)

	s.PushAndEmit(float64(signal))
	s.Slice = generalTruncate(s.Slice)
}

// GetSignal returns the latest signal (down, none or up)
func (s *UtBotAlertStream) GetSignal() types.Direction {
	return types.Direction(s.Last(0))
}

// Direction returns the signal at index i (0 is the latest)
func (s *UtBotAlertStream) Direction(i int) types.Direction {
	return types.Direction(s.Last(i))
}
