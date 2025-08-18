package risk

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestComputeVaRMonteCarlo(t *testing.T) {

	// GBM
	price_func := func(z float64) float64 {
		s := 100.0
		mu := 0.05
		sigma := 0.2
		dt := 1.0 / 252.0

		return s * math.Exp((mu-0.5*sigma*sigma)*dt+sigma*math.Sqrt(dt)*z)
	}

	out := ComputeVaRMonteCarlo(100.0, price_func, 0.95, time.Now().UnixNano(), 10000)
	assert.InDelta(t, out, -2.0, 0.2)
	out = ComputeVaRMonteCarlo(100.0, price_func, 0.05, time.Now().UnixNano(), 10000)
	assert.InDelta(t, out, 2.0, 0.2)

}

func TestComputeExpectedShortfall(t *testing.T) {

	// GBM
	price_func := func(z float64) float64 {
		s := 100.0
		mu := 0.05
		sigma := 0.2
		dt := 1.0 / 252.0

		return s * math.Exp((mu-0.5*sigma*sigma)*dt+sigma*math.Sqrt(dt)*z)
	}

	out := ComputeExpectedShortfall(100.0, price_func, 0.5, time.Now().UnixNano(), 10000)
	assert.InDelta(t, out, -1.0, 0.15)

}
