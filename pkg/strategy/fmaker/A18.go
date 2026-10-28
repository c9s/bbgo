package fmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// A18 is a fmaker factor calculated from the last `window` klines of the source.
type A18 struct {
	*kLineWindowFactor
}

func NewA18(source indicatorv2.KLineSubscription, window int) *A18 {
	return &A18{
		kLineWindowFactor: newKLineWindowFactor("A18", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateA18(kLines, types.KLineClosePriceMapper)
		}),
	}
}

// CLOSE/DELAY(CLOSE,5)
func calculateA18(klines []types.KLine, valClose KLineValueMapper) (float64, error) {
	window := 5
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var closes floats.Slice

	for _, k := range klines {
		closes.Push(valClose(k))
	}

	delay5 := closes.Last(4)
	curr := closes.Last(0)
	alpha := curr / delay5

	return alpha, nil
}
