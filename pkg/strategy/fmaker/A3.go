package fmaker

import (
	"fmt"
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// A3 is a fmaker factor calculated from the last `window` klines of the source.
type A3 struct {
	*kLineWindowFactor
}

func NewA3(source indicatorv2.KLineSubscription, window int) *A3 {
	return &A3{
		kLineWindowFactor: newKLineWindowFactor("A3", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateA3(kLines, KLineLowPriceMapper, KLineHighPriceMapper, types.KLineClosePriceMapper)
		}),
	}
}

// SUM((CLOSE = DELAY(CLOSE, 1)?0:CLOSE-(CLOSE>DELAY(CLOSE, 1)?MIN(LOW, DELAY(CLOSE, 1)):MAX(HIGH, DELAY(CLOSE, 1)))), 6)
func calculateA3(klines []types.KLine, valLow KLineValueMapper, valHigh KLineValueMapper, valClose KLineValueMapper) (float64, error) {
	window := 6 + 2
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

	a := 0.
	sumA := 0.
	for i := 1; i <= 6; i++ {
		if closes.Index(len(closes)-i) == closes.Index(len(closes)-i-1) {
			a = 0.
		} else {
			if closes.Index(len(closes)-i) > closes.Index(1) {
				a = closes.Index(len(closes)-i) - math.Min(lows.Index(len(lows)-i), closes.Index(len(closes)-i-1))
			} else {
				a = closes.Index(len(closes)-i) - math.Max(highs.Index(len(highs)-i), closes.Index(len(closes)-i-1))
			}
		}
		sumA += a
	}

	alpha := sumA // sum(a, 6 interval)

	return alpha, nil
}
