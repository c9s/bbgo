package fmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// S7 is a fmaker factor calculated from the last `window` klines of the source.
type S7 struct {
	*kLineWindowFactor
}

func NewS7(source indicatorv2.KLineSubscription, window int) *S7 {
	return &S7{
		kLineWindowFactor: newKLineWindowFactor("S7", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateS7(kLines, types.KLineOpenPriceMapper, types.KLineClosePriceMapper)
		}),
	}
}

func calculateS7(klines []types.KLine, valOpen KLineValueMapper, valClose KLineValueMapper) (float64, error) {
	window := 2
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var opens floats.Slice
	var closes floats.Slice

	for _, k := range klines {
		opens.Push(valOpen(k))
		closes.Push(valClose(k))

	}

	O := opens.Last(0)
	C := closes.Last(0)
	alpha := -(1 - O/C)

	return alpha, nil
}
