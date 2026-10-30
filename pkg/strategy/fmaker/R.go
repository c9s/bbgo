package fmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// R is a fmaker factor calculated from the last `window` klines of the source.
type R struct {
	*kLineWindowFactor
}

func NewR(source indicatorv2.KLineSubscription, window int) *R {
	return &R{
		kLineWindowFactor: newKLineWindowFactor("R", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateR(kLines, types.KLineOpenPriceMapper, types.KLineClosePriceMapper)
		}),
	}
}

func calculateR(klines []types.KLine, valOpen KLineValueMapper, valClose KLineValueMapper) (float64, error) {
	window := 1
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

	ret := opens.Index(0)/closes.Index(0) - 1 // delta(1 interval)

	return ret, nil
}
