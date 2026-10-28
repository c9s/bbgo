package fmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// S5 is a fmaker factor calculated from the last `window` klines of the source.
type S5 struct {
	*kLineWindowFactor
}

func NewS5(source indicatorv2.KLineSubscription, window int) *S5 {
	return &S5{
		kLineWindowFactor: newKLineWindowFactor("S5", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateS5(kLines, types.KLineVolumeMapper)
		}),
	}
}

func calculateS5(klines []types.KLine, valVolume KLineValueMapper) (float64, error) {
	window := 10
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var volumes floats.Slice

	for _, k := range klines {
		volumes.Push(valVolume(k))
	}

	v := volumes.Last(0)

	sumV := 0.
	for i := 1; i <= 10; i++ {
		sumV += volumes.Index(len(volumes) - i)
	}

	meanV := sumV / 10

	alpha := -v / meanV

	return alpha, nil
}
