package indicatorv2

import (
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

type SuperTrendBand struct {
	UppderBand float64
	LowerBand  float64
	Direction  int // 1 for uptrend, -1 for downtrend
	Time       time.Time
}

func (s *SuperTrendBand) Band() float64 {
	if s.Direction == 1 {
		return s.LowerBand
	}
	return s.UppderBand
}

type SuperTrendStream struct {
	*types.Float64Series

	source     KLineSubscription
	atr        *ATRStream
	multiplier float64
	entities   []*SuperTrendBand
}

func (st *SuperTrendStream) update(kline types.KLine) {
	currentATR := st.atr.Last(0)
	if currentATR == 0 {
		// not enough data, skip
		return
	}
	hl2 := (kline.High.Float64() + kline.Low.Float64()) / 2
	basicUpperBand := hl2 + st.multiplier*currentATR
	basicLowerBand := hl2 - st.multiplier*currentATR

	prevUpperBand := 0.0
	prevLowerBand := 0.0
	prevClose := st.source.Last(1).Close.Float64()
	prevDirection := 1
	if len(st.entities) > 0 {
		prevEntity := st.entities[len(st.entities)-1]
		prevUpperBand = prevEntity.UppderBand
		prevLowerBand = prevEntity.LowerBand
		prevDirection = st.entities[len(st.entities)-1].Direction
	}

	// final upper band and lower band calculation
	// if we have a lower/higher basic band or the previous close crosses the bands -> adjust the bands
	finalUpperBand := prevUpperBand
	finalLowerBand := prevLowerBand
	if basicUpperBand < prevUpperBand || prevClose > prevUpperBand {
		finalUpperBand = basicUpperBand
	}
	if basicLowerBand > prevLowerBand || prevClose < prevLowerBand {
		finalLowerBand = basicLowerBand
	}

	// trend direction
	direction := 1
	currentClose := kline.Close.Float64()
	if prevDirection == 1 {
		// was bullish
		if currentClose < finalLowerBand {
			// switch to bearish
			direction = -1
		} else {
			// stay bullish
			direction = 1
		}
	} else {
		// was bearish
		if currentClose > finalUpperBand {
			// switch to bullish
			direction = 1
		} else {
			// stay bearish
			direction = -1
		}
	}

	e := &SuperTrendBand{
		UppderBand: finalUpperBand,
		LowerBand:  finalLowerBand,
		Direction:  direction,
		Time:       kline.EndTime.Time(),
	}
	st.entities = append(st.entities, e)
	st.PushAndEmit(e.Band())
}

func SuperTrend(source KLineSubscription, window int, multiplier float64) *SuperTrendStream {
	atr := ATR2(source, window)
	st := &SuperTrendStream{
		Float64Series: types.NewFloat64Series(),
		source:        source,
		atr:           atr,
		multiplier:    multiplier,
		entities:      make([]*SuperTrendBand, 0),
	}
	source.AddSubscriber(st.update)
	return st
}
