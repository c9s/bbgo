package fmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// A2 is a fmaker factor calculated from the last `window` klines of the source.
type A2 struct {
	*kLineWindowFactor
}

func NewA2(source indicatorv2.KLineSubscription, window int) *A2 {
	return &A2{
		kLineWindowFactor: newKLineWindowFactor("A2", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateA2(kLines, KLineLowPriceMapper, KLineHighPriceMapper, types.KLineClosePriceMapper)
		}),
	}
}

// (-1 * DELTA((((CLOSE - LOW) - (HIGH - CLOSE)) / (HIGH - LOW)), 1))
func calculateA2(klines []types.KLine, valLow KLineValueMapper, valHigh KLineValueMapper, valClose KLineValueMapper) (float64, error) {
	window := 2
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var lows floats.Slice
	var highs floats.Slice
	var closes floats.Slice

	for _, k := range klines {
		lows.Push(valLow(k))
		highs.Push(valHigh(k))
		closes.Push(valClose(k))
	}

	prev := ((closes.Last(1) - lows.Index(1)) - (highs.Index(1) - closes.Index(1))) / (highs.Index(1) - lows.Index(1))
	curr := ((closes.Last(0) - lows.Index(0)) - (highs.Index(0) - closes.Index(0))) / (highs.Index(0) - lows.Index(0))
	alpha := (curr - prev) * -1 // delta(1 interval)

	return alpha, nil
}

func KLineLowPriceMapper(k types.KLine) float64 {
	return k.Low.Float64()
}

func KLineHighPriceMapper(k types.KLine) float64 {
	return k.High.Float64()
}
