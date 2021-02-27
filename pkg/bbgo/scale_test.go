package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestExpScale(t *testing.T) {
	// graph see: https://www.desmos.com/calculator/ip0ijbcbbf
	scale := ExpScale{
		Domain: [2]float64{1000, 2000},
		Range:  [2]float64{0.001, 0.01},
	}

	err := scale.Solve()
	assert.NoError(t, err)

	assert.Equal(t, "f(x) = 0.001000 * 1.002305 ^ (x - 1000.000000)", scale.String())
	assert.Equal(t, fixedpoint.NewFromFloat(0.001), fixedpoint.NewFromFloat(scale.Call(1000.0)))
	assert.Equal(t, fixedpoint.NewFromFloat(0.01), fixedpoint.NewFromFloat(scale.Call(2000.0)))

	for x := 1000; x <= 2000; x += 100 {
		y := scale.Call(float64(x))
		t.Logf("%s = %f", scale.FormulaOf(float64(x)), y)
	}
}

func TestLogScale(t *testing.T) {
	scale := LogScale{
		Domain: [2]float64{1000, 2000},
		Range:  [2]float64{0.001, 0.01},
	}

	err := scale.Solve()
	assert.NoError(t, err)
	assert.Equal(t, "f(x) = 0.001303 * log(x - 999.000000) + 0.001000", scale.String())
	assert.Equal(t, fixedpoint.NewFromFloat(0.001), fixedpoint.NewFromFloat(scale.Call(1000.0)))
	assert.Equal(t, fixedpoint.NewFromFloat(0.01), fixedpoint.NewFromFloat(scale.Call(2000.0)))
	for x := 1000; x <= 2000; x += 100 {
		y := scale.Call(float64(x))
		t.Logf("%s = %f", scale.FormulaOf(float64(x)), y)
	}
}
