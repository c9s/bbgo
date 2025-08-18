package floats

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gonum.org/v1/gonum/stat/distuv"
)

func TestNewRandomNormal(t *testing.T) {
	a := NewRandomNormal(5, 1, 1000)
	assert.Equal(t, 1000, len(a))
	mean := a.Mean()
	assert.InDelta(t, 5, mean, 0.2)
	std := distuv.Normal{
		Mu:    5,
		Sigma: 1,
	}.StdDev()
	assert.InDelta(t, std, a.Std(), 0.2)
}

func TestNewRandomPoisson(t *testing.T) {
	a := NewRandomPoisson(5, 1000)
	assert.Equal(t, 1000, len(a))
	mean := a.Mean()
	assert.InDelta(t, 5, mean, 0.2)
	std := distuv.Poisson{
		Lambda: 5,
	}.StdDev()
	assert.InDelta(t, std, a.Std(), 0.2)
}

func TestNewRandomUniform(t *testing.T) {
	a := NewRandomUniform(1, 10, 1000)
	assert.Equal(t, 1000, len(a))
	mean := a.Mean()
	assert.InDelta(t, 5.5, mean, 0.2)
	std := distuv.Uniform{
		Min: 1,
		Max: 10,
	}.StdDev()
	assert.InDelta(t, std, a.Std(), 0.2)
}

func TestSub(t *testing.T) {
	a := New(1, 2, 3, 4, 5)
	b := New(1, 2, 3, 4, 5)
	c := a.Sub(b)
	assert.Equal(t, Slice{.0, .0, .0, .0, .0}, c)
	assert.Equal(t, 5, len(c))
	assert.Equal(t, 5, c.Length())
}

func TestTruncate(t *testing.T) {
	a := New(1, 2, 3, 4, 5)
	for i := 5; i > 0; i-- {
		a = a.Truncate(i)
		assert.Equal(t, i, a.Length())
	}
}

func TestAdd(t *testing.T) {
	a := New(1, 2, 3, 4, 5)
	b := New(1, 2, 3, 4, 5)
	c := a.Add(b)
	assert.Equal(t, Slice{2.0, 4.0, 6.0, 8.0, 10.0}, c)
	assert.Equal(t, 5, len(c))
	assert.Equal(t, 5, c.Length())
}
