package floats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPercentile(t *testing.T) {
	out := Percentile([]float64{10.0, 11.0, 12.0, 13.0, 15.0}, 50.0)
	assert.Equal(t, 12.0, out)

	out = Percentile([]float64{1.0, 3.0}, 50.0)
	assert.Equal(t, 2.0, out)

	out = Percentile([]float64{0.0, 1.0}, 25.0)
	assert.Equal(t, 0.25, out)

	out = Percentile([]float64{0.0, 1.0, 10}, 25.0)
	assert.Equal(t, 0.5, out)

}

func TestLower(t *testing.T) {
	out := Lower([]float64{10.0, 11.0, 12.0, 13.0, 15.0}, 12.0)
	assert.Equal(t, []float64{10.0, 11.0}, out)
}

func TestHigher(t *testing.T) {
	out := Higher([]float64{10.0, 11.0, 12.0, 13.0, 15.0}, 12.0)
	assert.Equal(t, []float64{13.0, 15.0}, out)
}

func TestLSM(t *testing.T) {
	slice := Slice{1., 2., 3., 4.}
	slope := LSM(slice)
	assert.Equal(t, 1.0, slope)
}
