package bbgo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

const delta = 1e-9

func TestLayerScale_UnmarshalJSON(t *testing.T) {
	var s LayerScale
	err := json.Unmarshal([]byte(`{
		"byLayer": {
			"linear": {
				"domain": [ 1, 3 ],
				"range": [ 10000.0, 30000.0 ]
			}
		}
	}`), &s)
	assert.NoError(t, err)

	if assert.NotNil(t, s.LayerRule) {
		assert.NotNil(t, s.LayerRule.LinearScale.Range)
		assert.NotNil(t, s.LayerRule.LinearScale.Domain)
	}
}

func TestExponentialScale(t *testing.T) {
	// graph see: https://www.desmos.com/calculator/ip0ijbcbbf
	scale := ExponentialScale{
		Domain: [2]float64{1000, 2000},
		Range:  [2]float64{0.001, 0.01},
	}

	err := scale.Solve()
	assert.NoError(t, err)

	assert.Equal(t, "f(x) = 0.001000 * 1.002305 ^ (x - 1000.000000)", scale.String())
	assert.InDelta(t, 0.001, scale.Call(1000.0), delta)
	assert.InDelta(t, 0.01, scale.Call(2000.0), delta)

	for x := 1000; x <= 2000; x += 100 {
		y := scale.Call(float64(x))
		t.Logf("%s = %f", scale.FormulaOf(float64(x)), y)
	}
}

func TestExponentialScale_Reverse(t *testing.T) {
	scale := ExponentialScale{
		Domain: [2]float64{1000, 2000},
		Range:  [2]float64{0.1, 0.001},
	}

	err := scale.Solve()
	assert.NoError(t, err)

	assert.Equal(t, "f(x) = 0.100000 * 0.995405 ^ (x - 1000.000000)", scale.String())
	assert.InDelta(t, 0.1, scale.Call(1000.0), delta)
	assert.InDelta(t, 0.001, scale.Call(2000.0), delta)

	for x := 1000; x <= 2000; x += 100 {
		y := scale.Call(float64(x))
		t.Logf("%s = %f", scale.FormulaOf(float64(x)), y)
	}
}

func TestLogScale(t *testing.T) {
	// see https://www.desmos.com/calculator/q1ufxx5gry
	scale := LogarithmicScale{
		Domain: [2]float64{1000, 2000},
		Range:  [2]float64{0.001, 0.01},
	}

	err := scale.Solve()
	assert.NoError(t, err)
	assert.Equal(t, "f(x) = 0.001303 * log(x - 999.000000) + 0.001000", scale.String())
	assert.InDelta(t, 0.001, scale.Call(1000.0), delta)
	assert.InDelta(t, 0.01, scale.Call(2000.0), delta)
	for x := 1000; x <= 2000; x += 100 {
		y := scale.Call(float64(x))
		t.Logf("%s = %f", scale.FormulaOf(float64(x)), y)
	}
}

func TestLinearScale(t *testing.T) {
	scale := LinearScale{
		Domain: [2]float64{1000, 2000},
		Range:  [2]float64{3, 10},
	}

	err := scale.Solve()
	assert.NoError(t, err)
	assert.Equal(t, "f(x) = 3.000000 + (x - 1000.000000) * 0.007000", scale.String())
	assert.InDelta(t, 3, scale.Call(1000), delta)
	assert.InDelta(t, 6.5, scale.Call(1500), delta)
	assert.InDelta(t, 10, scale.Call(2000), delta)
	for x := 1000; x <= 2000; x += 100 {
		y := scale.Call(float64(x))
		t.Logf("%s = %f", scale.FormulaOf(float64(x)), y)
	}
}

func TestLinearScale2(t *testing.T) {
	scale := LinearScale{
		Domain: [2]float64{1, 3},
		Range:  [2]float64{0.1, 0.4},
	}

	err := scale.Solve()
	assert.NoError(t, err)
	assert.Equal(t, "f(x) = 0.100000 + (x - 1.000000) * 0.150000", scale.String())
	assert.InDelta(t, 0.1, scale.Call(1), delta)
	assert.InDelta(t, 0.25, scale.Call(2), delta)
	assert.InDelta(t, 0.4, scale.Call(3), delta)
}

func TestLinearScaleNegative(t *testing.T) {
	scale := LinearScale{
		Domain: [2]float64{-1, 3},
		Range:  [2]float64{0.1, 0.4},
	}

	err := scale.Solve()
	assert.NoError(t, err)
	assert.Equal(t, "f(x) = 0.100000 + (x - -1.000000) * 0.075000", scale.String())
	assert.InDelta(t, 0.1, scale.Call(-1), delta)
	assert.InDelta(t, 0.25, scale.Call(1), delta)
	assert.InDelta(t, 0.4, scale.Call(3), delta)
}

func TestQuadraticScale(t *testing.T) {
	// see https://www.desmos.com/calculator/vfqntrxzpr
	scale := QuadraticScale{
		Domain: [3]float64{0, 100, 200},
		Range:  [3]float64{1, 20, 50},
	}

	err := scale.Solve()
	assert.NoError(t, err)
	assert.Equal(t, "f(x) = 0.000550 * x ^ 2 + 0.135000 * x + 1.000000", scale.String())
	assert.InDelta(t, 1, scale.Call(0), delta)
	assert.InDelta(t, 20, scale.Call(100.0), delta)
	assert.InDelta(t, 50.0, scale.Call(200.0), delta)
	for x := 0; x <= 200; x += 1 {
		y := scale.Call(float64(x))
		t.Logf("%s = %f", scale.FormulaOf(float64(x)), y)
	}
}

func TestPercentageScale(t *testing.T) {
	t.Run("from 0.0 to 1.0", func(t *testing.T) {
		s := &PercentageScale{
			ByPercentage: &SlideRule{
				ExpScale: &ExponentialScale{
					Domain: [2]float64{0.0, 1.0},
					Range:  [2]float64{1.0, 100.0},
				},
			},
		}

		v, err := s.Scale(0.0)
		assert.NoError(t, err)
		assert.InDelta(t, 1.0, v, delta)

		v, err = s.Scale(1.0)
		assert.NoError(t, err)
		assert.InDelta(t, 100.0, v, delta)
	})

	t.Run("from -1.0 to 1.0", func(t *testing.T) {
		s := &PercentageScale{
			ByPercentage: &SlideRule{
				ExpScale: &ExponentialScale{
					Domain: [2]float64{-1.0, 1.0},
					Range:  [2]float64{10.0, 100.0},
				},
			},
		}

		v, err := s.Scale(-1.0)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0, v, delta)

		v, err = s.Scale(1.0)
		assert.NoError(t, err)
		assert.InDelta(t, 100.0, v, delta)
	})

	t.Run("reverse -1.0 to 1.0", func(t *testing.T) {
		s := &PercentageScale{
			ByPercentage: &SlideRule{
				ExpScale: &ExponentialScale{
					Domain: [2]float64{-1.0, 1.0},
					Range:  [2]float64{100.0, 10.0},
				},
			},
		}

		v, err := s.Scale(-1.0)
		assert.NoError(t, err)
		assert.InDelta(t, 100.0, v, delta)

		v, err = s.Scale(1.0)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0, v, delta)

		v, err = s.Scale(2.0)
		assert.NoError(t, err)
		assert.InDelta(t, 10.0, v, delta)

		v, err = s.Scale(-2.0)
		assert.NoError(t, err)
		assert.InDelta(t, 100.0, v, delta)
	})

	t.Run("negative range", func(t *testing.T) {
		s := &PercentageScale{
			ByPercentage: &SlideRule{
				ExpScale: &ExponentialScale{
					Domain: [2]float64{0.0, 1.0},
					Range:  [2]float64{-100.0, 100.0},
				},
			},
		}

		v, err := s.Scale(0.0)
		assert.NoError(t, err)
		assert.InDelta(t, -100.0, v, delta)

		v, err = s.Scale(1.0)
		assert.NoError(t, err)
		assert.InDelta(t, 100.0, v, delta)
	})
}
