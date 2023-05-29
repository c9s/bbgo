package floats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
