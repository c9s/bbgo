package fmaker

import (
	"fmt"
	"math"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// S2 is a fmaker factor calculated from the last `window` klines of the source.
type S2 struct {
	*kLineWindowFactor
}

func NewS2(source indicatorv2.KLineSubscription, window int) *S2 {
	return &S2{
		kLineWindowFactor: newKLineWindowFactor("S2", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateS2(kLines, window, types.KLineOpenPriceMapper, types.KLineVolumeMapper)
		}),
	}
}

func calculateS2(klines []types.KLine, window int, valA KLineValueMapper, valB KLineValueMapper) (float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
	}

	sumA, sumB, sumAB, squareSumA, squareSumB := 0., 0., 0., 0., 0.
	for _, k := range klines {
		// sum of elements of array A
		sumA += valA(k)
		// sum of elements of array B
		sumB += valB(k)

		// sum of A[i] * B[i].
		sumAB = sumAB + valA(k)*valB(k)

		// sum of square of array elements.
		squareSumA = squareSumA + valA(k)*valA(k)
		squareSumB = squareSumB + valB(k)*valB(k)
	}
	// use formula for calculating correlation coefficient.
	corr := (float64(window)*sumAB - sumA*sumB) /
		math.Sqrt((float64(window)*squareSumA-sumA*sumA)*(float64(window)*squareSumB-sumB*sumB))

	return -corr, nil
}
