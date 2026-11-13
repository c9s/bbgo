package fmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// S6 is a fmaker factor calculated from the last `window` klines of the source.
type S6 struct {
	*kLineWindowFactor
}

func NewS6(source indicatorv2.KLineSubscription, window int) *S6 {
	return &S6{
		kLineWindowFactor: newKLineWindowFactor("S6", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateS6(kLines, types.KLineHighPriceMapper, types.KLineLowPriceMapper, types.KLineClosePriceMapper, types.KLineVolumeMapper)
		}),
	}
}

func calculateS6(klines []types.KLine, valHigh KLineValueMapper, valLow KLineValueMapper, valClose KLineValueMapper, valVolume KLineValueMapper) (float64, error) {
	window := 2
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var highs floats.Slice
	var lows floats.Slice
	var closes floats.Slice
	var volumes floats.Slice

	for _, k := range klines {
		highs.Push(valHigh(k))
		lows.Push(valLow(k))
		closes.Push(valClose(k))
		volumes.Push(valVolume(k))

	}

	H := highs.Last(0)
	L := lows.Last(0)
	C := closes.Last(0)
	V := volumes.Last(0)
	alpha := (H + L + C) / 3 * V

	return alpha, nil
}
