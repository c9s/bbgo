package fmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// A34 is a fmaker factor calculated from the last `window` klines of the source.
type A34 struct {
	*kLineWindowFactor
}

func NewA34(source indicatorv2.KLineSubscription, window int) *A34 {
	return &A34{
		kLineWindowFactor: newKLineWindowFactor("A34", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateA34(kLines, types.KLineClosePriceMapper)
		}),
	}
}

func calculateA34(klines []types.KLine, valClose KLineValueMapper) (float64, error) {
	window := 12
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var closes floats.Slice

	for _, k := range klines {
		closes.Push(valClose(k))
	}

	c := closes.Last(0)

	sumC := 0.
	for i := 1; i <= 12; i++ {
		sumC += closes.Index(len(closes) - i)
	}

	meanC := sumC / 12

	alpha := meanC / c

	return alpha, nil
}
