package indicatorv2

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Chandelier Exit Stop
// Refer URL: https://www.tradingview.com/script/mjBdRGXe-Chandelier-Stop/
// Chandelier Stop is to trail the stop loss based on ATR and Highest High / Lowest Low
//
// Long Stop = Highest High - (ATR Multiplier * ATR)
// Short Stop = Lowest Low + (ATR Multiplier * ATR)
//
// Where Highest High and Lowest Low are calculated over a specified length period.
//
// It also detect the direction switches:
// - When the previous close is below the previous short stop and the current close is above the current short stop, it switches to long direction.
// - When the previous close is above the previous long stop and the current close is below the current long stop, it switches to short direction.
// - When there is no switch, the direction remains the same as the previous direction. Can be long, short, or neutral (0).

type ChandelierStopPrice struct {
	LongStop  float64
	ShortStop float64
	Direction int // 1 for long, -1 for short, 0 for neutral
	Switched  bool
	Time      time.Time
}

func (c *ChandelierStopPrice) String() string {
	var stopPrice float64
	if c.Direction > 0 {
		stopPrice = c.LongStop
	} else {
		stopPrice = c.ShortStop
	}
	return fmt.Sprintf("%0.4f (Direction: %d)", stopPrice, c.Direction)
}

type ChandelierStopStream struct {
	*types.Float64Series

	high, low, close *PriceStream
	atr              *ATRStream
	length           int
	multiplier       float64
	emitDirection    bool

	// buffers
	longStops  []float64
	shortStops []float64
	stopPrices []ChandelierStopPrice
}

func (cs *ChandelierStopStream) StopPrices() []ChandelierStopPrice {
	return cs.stopPrices
}

func (cs *ChandelierStopStream) update(kline types.KLine) {
	if cs.high.Length() < cs.length || cs.low.Length() < cs.length {
		// not enough data
		return
	}
	hh := cs.high.Highest(cs.length)
	ll := cs.low.Lowest(cs.length)
	currentClose := cs.close.Last(0)
	currentAtr := cs.atr.Last(0)

	longStopRaw := hh - cs.multiplier*currentAtr
	shortStopRaw := ll + cs.multiplier*currentAtr

	var prevLongStop, prevShortStop float64
	if len(cs.longStops) == 0 {
		prevLongStop = 0
		prevShortStop = 0
	} else {
		prevLongStop = cs.longStops[len(cs.longStops)-1]
		prevShortStop = cs.shortStops[len(cs.shortStops)-1]
	}

	var longStop, shortStop float64
	if currentClose < prevLongStop {
		// current close is below previous long stop -> reset long stop
		longStop = longStopRaw
	} else {
		// otherwise trail the stop upwards
		longStop = max(prevLongStop, longStopRaw)
	}

	if currentClose > prevShortStop {
		// current close is above previous short stop -> reset short stop
		shortStop = shortStopRaw
	} else {
		// otherwise trail the stop downwards
		shortStop = min(prevShortStop, shortStopRaw)
	}
	cs.longStops = append(cs.longStops, longStop)
	cs.shortStops = append(cs.shortStops, shortStop)

	// direction switches
	prevClose := cs.close.Last(1)
	longSwitch := prevClose < prevShortStop && currentClose >= prevShortStop
	shortSwitch := prevClose > prevLongStop && currentClose <= prevLongStop

	lastDirection := 0
	if len(cs.stopPrices) > 0 {
		lastDirection = cs.stopPrices[len(cs.stopPrices)-1].Direction
	}
	// update direction
	direction := lastDirection
	if lastDirection <= 0 && longSwitch {
		// switch to long
		direction = 1
	} else if lastDirection >= 0 && shortSwitch {
		// switch to short
		direction = -1
	}

	sp := ChandelierStopPrice{
		LongStop:  longStop,
		ShortStop: shortStop,
		Direction: direction,
		Switched:  longSwitch || shortSwitch,
		Time:      kline.StartTime.Time(),
	}
	cs.stopPrices = append(cs.stopPrices, sp)

	// push and emit value
	var ev float64
	if cs.emitDirection {
		ev = float64(sp.Direction)
	} else {
		if sp.Direction > 0 {
			ev = sp.LongStop
		} else {
			ev = sp.ShortStop
		}
	}
	cs.PushAndEmit(ev)

	// shrink slices if necessary
	cs.longStops = types.ShrinkSlice(cs.longStops, 800, 400)
	cs.shortStops = types.ShrinkSlice(cs.shortStops, 800, 400)
	cs.stopPrices = types.ShrinkSlice(cs.stopPrices, 800, 400)
}

func ChandelierStop(source KLineSubscription, length, atrPeriod int, multiplier float64, emitDirection bool) *ChandelierStopStream {
	cs := &ChandelierStopStream{
		Float64Series: types.NewFloat64Series(),

		high:          HighPrices(source),
		low:           LowPrices(source),
		close:         ClosePrices(source),
		atr:           ATR2(source, atrPeriod),
		length:        length,
		multiplier:    multiplier,
		emitDirection: emitDirection,
	}
	source.AddSubscriber(cs.update)
	return cs
}
