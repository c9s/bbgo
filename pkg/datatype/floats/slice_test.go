package floats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSub(t *testing.T) {
	a := New(1, 2, 3, 4, 5)
	b := New(1, 2, 3, 4, 5)
	c := a.Sub(b)
	assert.Equal(t, Slice{.0, .0, .0, .0, .0}, c)
	assert.Equal(t, 5, len(c))
	assert.Equal(t, 5, c.Length())
}

func TestAdd(t *testing.T) {
	a := New(1, 2, 3, 4, 5)
	b := New(1, 2, 3, 4, 5)
	c := a.Add(b)
	assert.Equal(t, Slice{2.0, 4.0, 6.0, 8.0, 10.0}, c)
	assert.Equal(t, 5, len(c))
	assert.Equal(t, 5, c.Length())
}
