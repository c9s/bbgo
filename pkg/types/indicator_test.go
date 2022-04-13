package types

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFloat(t *testing.T) {
	var a Series = Minus(3., 2.)
	assert.Equal(t, a.Last(), 1.)
	assert.Equal(t, a.Index(100), 1.)
}

func TestNextCross(t *testing.T) {
	var a Series = NumberSeries(1.2)

	var b Series = &Float64Slice{100., 80., 60.}
	// index                       2    1    0
	// predicted                                40  20  0
	// offset                                   1   2   3

	index, value, ok := NextCross(a, b, 3)
	assert.True(t, ok)
	assert.Equal(t, value, 1.2)
	assert.Equal(t, index, 3) // 2.94, ceil
}

func TestFloat64Slice(t *testing.T) {
	var a = Float64Slice{1.0, 2.0, 3.0}
	var b = Float64Slice{1.0, 2.0, 3.0}
	var c Series = Minus(&a, &b)
	a = append(a, 4.0)
	b = append(b, 3.0)
	assert.Equal(t, c.Last(), 1.)
}
