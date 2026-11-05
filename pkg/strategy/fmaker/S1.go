package fmaker

import (
	"fmt"
	"math"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// S1 is a fmaker factor calculated from the last `window` klines of the source.
type S1 struct {
	*kLineWindowFactor
}

func NewS1(source indicatorv2.KLineSubscription, window int) *S1 {
	return &S1{
		kLineWindowFactor: newKLineWindowFactor("S1", source, window, func(kLines []types.KLine) (float64, error) {
			return calculateS1(kLines, window, KLineAmplitudeMapper, types.KLineVolumeMapper)
		}),
	}
}

func calculateS1(klines []types.KLine, window int, valA KLineValueMapper, valB KLineValueMapper) (float64, error) {
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

func KLineAmplitudeMapper(k types.KLine) float64 {
	return k.High.Div(k.Low).Float64()
}
