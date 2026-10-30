package fmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// S0 is a fmaker factor calculated from the last `window` klines of the source.
type S0 struct {
	*kLineWindowFactor
}

func NewS0(source indicatorv2.KLineSubscription, window int) *S0 {
	return &S0{
		kLineWindowFactor: newKLineWindowFactor("S0", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateS0(kLines, types.KLineClosePriceMapper)
		}),
	}
}

func calculateS0(klines []types.KLine, valClose KLineValueMapper) (float64, error) {
	window := 20
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var closes floats.Slice

	for _, k := range klines {
		closes.Push(valClose(k))
	}

	sma := floats.Slice.Sum(closes[len(closes)-window:len(closes)-1]) / float64(window)
	alpha := sma / closes.Last(0)

	return alpha, nil
}
