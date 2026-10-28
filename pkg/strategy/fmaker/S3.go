package fmaker

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// S3 is a fmaker factor calculated from the last `window` klines of the source.
type S3 struct {
	*kLineWindowFactor
}

func NewS3(source indicatorv2.KLineSubscription, window int) *S3 {
	return &S3{
		kLineWindowFactor: newKLineWindowFactor("S3", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateS3(kLines, types.KLineClosePriceMapper, types.KLineOpenPriceMapper)
		}),
	}
}

func calculateS3(klines []types.KLine, valClose KLineValueMapper, valOpen KLineValueMapper) (float64, error) {
	window := 2
	length := len(klines)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating  with window = %d", window)
	}
	var closes floats.Slice
	var opens floats.Slice

	for _, k := range klines {
		closes.Push(valClose(k))
		opens.Push(valOpen(k))
	}

	prevC := closes.Index(1)
	currO := opens.Index(0)
	alpha := currO / prevC

	return alpha, nil
}
