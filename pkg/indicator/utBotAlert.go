package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/bools"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// based on "UT Bot Alerts by QuantNomad" from tradingview

//go:generate callbackgen -type UtBotAlert
type UtBotAlert struct {
	types.IntervalWindow
	KeyValue float64 `json:"keyValue"` // Should be ATRMultiplier

	Values    []types.Direction
	buyValue  bools.BoolSlice
	sellValue bools.BoolSlice

	AverageTrueRange *ATR // Value must be set when initialized in strategy

	xATRTrailingStop floats.Slice
	pos              types.Direction // NB: This is currently not in use (kept in case of expanding as it is in the tradingview version)
	previousPos      types.Direction // NB: This is currently not in use (kept in case of expanding as it is in the tradingview version)

	previousClosePrice float64

	EndTime         time.Time
	UpdateCallbacks []func(value types.Direction)
}

func NewUtBotAlert(iw types.IntervalWindow, keyValue float64) *UtBotAlert {
	return &UtBotAlert{
		IntervalWindow: iw,
		KeyValue:       keyValue,
		AverageTrueRange: &ATR{
			IntervalWindow: iw,
		},
	}
}

func (inc *UtBotAlert) Last() types.Direction {
	length := len(inc.Values)
	if length > 0 {
		return inc.Values[length-1]
	}
	return types.DirectionNone
}

func (inc *UtBotAlert) Index(i int) types.Direction {
	length := inc.Length()
	if length == 0 || length-i-1 < 0 {
		return 0
	}
	return inc.Values[length-i-1]
}

func (inc *UtBotAlert) Length() int {
	return len(inc.Values)
}

func (inc *UtBotAlert) Update(highPrice, lowPrice, closePrice float64) {
	if inc.Window <= 0 {
		panic("window must be greater than 0")
	}

	// Update ATR
	inc.AverageTrueRange.Update(highPrice, lowPrice, closePrice)

	nLoss := inc.AverageTrueRange.Last(0) * inc.KeyValue

	// xATRTrailingStop
	if inc.xATRTrailingStop.Length() == 0 {
		// For first run
		inc.xATRTrailingStop.Update(0)

	} else if closePrice > inc.xATRTrailingStop.Last(1) && inc.previousClosePrice > inc.xATRTrailingStop.Last(1) {
		inc.xATRTrailingStop.Update(math.Max(inc.xATRTrailingStop.Last(1), closePrice-nLoss))

	} else if closePrice < inc.xATRTrailingStop.Last(1) && inc.previousClosePrice < inc.xATRTrailingStop.Last(1) {
		inc.xATRTrailingStop.Update(math.Min(inc.xATRTrailingStop.Last(1), closePrice+nLoss))

	} else if closePrice > inc.xATRTrailingStop.Last(1) {
		inc.xATRTrailingStop.Update(closePrice - nLoss)

	} else {
		inc.xATRTrailingStop.Update(closePrice + nLoss)
	}

	// pos
	if inc.previousClosePrice < inc.xATRTrailingStop.Last(1) && closePrice > inc.xATRTrailingStop.Last(1) {
		inc.pos = types.DirectionUp
	} else if inc.previousClosePrice > inc.xATRTrailingStop.Last(1) && closePrice < inc.xATRTrailingStop.Last(1) {
		inc.pos = types.DirectionDown
	} else {
		inc.pos = inc.previousPos
	}

	above := closePrice > inc.xATRTrailingStop.Last(0) && inc.previousClosePrice < inc.xATRTrailingStop.Last(1)
	below := closePrice < inc.xATRTrailingStop.Last(0) && inc.previousClosePrice > inc.xATRTrailingStop.Last(1)

	buy := closePrice > inc.xATRTrailingStop.Last(0) && above  // buy
	sell := closePrice < inc.xATRTrailingStop.Last(0) && below // sell

	inc.buyValue.Push(buy)
	inc.sellValue.Push(sell)

	if buy {
		inc.Values = append(inc.Values, types.DirectionUp)
	} else if sell {
		inc.Values = append(inc.Values, types.DirectionDown)
	} else {
		inc.Values = append(inc.Values, types.DirectionNone)
	}

	// Update last prices
	inc.previousClosePrice = closePrice
	inc.previousPos = inc.pos

}

// GetSignal returns signal (down, none or up)
func (inc *UtBotAlert) GetSignal() types.Direction {
	length := len(inc.Values)
	if length > 0 {
		return inc.Values[length-1]
	}
	return types.DirectionNone
}

func (inc *UtBotAlert) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
		return
	}

	inc.Update(k.GetHigh().Float64(), k.GetLow().Float64(), k.GetClose().Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last())

}

func (inc *UtBotAlert) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

// LoadK calculates the initial values
func (inc *UtBotAlert) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
}
