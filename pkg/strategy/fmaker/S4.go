package fmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// S4 is a fmaker factor calculated from the last `window` klines of the source.
type S4 struct {
	*kLineWindowFactor
}

func NewS4(source indicatorv2.KLineSubscription, window int) *S4 {
	return &S4{
		kLineWindowFactor: newKLineWindowFactor("S4", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateS4(kLines, types.KLineClosePriceMapper)
		}),
	}
}

func calculateS4(klines []types.KLine, valClose KLineValueMapper) (float64, error) {
	window := 2
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var closes floats.Slice

	for _, k := range klines {
		closes.Push(valClose(k))
	}

	currC := closes.Index(0)
	alpha := 1 / currC

	return alpha, nil
}
