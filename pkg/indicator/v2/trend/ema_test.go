package trend

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v2 "github.com/c9s/bbgo/pkg/indicator/v2"
)

func TestExponentialMovingAverage(t *testing.T) {
	closing := []float64{
		64.75, 63.79, 63.73,
		63.73, 63.55, 63.19,
		63.91, 63.85, 62.95,
		63.37, 61.33, 61.51}
	expected := []float64{
		64.75, 64.37, 64.11,
		63.96, 63.8, 63.55,
		63.7, 63.76, 63.43,
		63.41, 62.58, 62.15,
	}
	prices := v2.ClosePrices(nil)
	ind := EWMA2(prices, 4)

	for _, price := range closing {
		prices.PushAndEmit(price)
	}
	for i, v := range expected {
		assert.InDelta(t, v, ind.Slice[i], 0.01, "Expected EWMA.slice[%d] to be %v, but got %v", i, v, ind.Slice[i])
	}

}
