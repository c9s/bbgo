package fixedpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ValueMap_Eq(t *testing.T) {
	m1 := ValueMap{
		"A": NewFromFloat(3.0),
		"B": NewFromFloat(4.0),
	}

	m2 := ValueMap{}

	m3 := ValueMap{"A": NewFromFloat(5.0)}

	m4 := ValueMap{
		"A": NewFromFloat(6.0),
		"B": NewFromFloat(7.0),
	}

	m5 := ValueMap{
		"A": NewFromFloat(3.0),
		"B": NewFromFloat(4.0),
	}

	assert.True(t, m1.Eq(m1))
	assert.False(t, m1.Eq(m2))
	assert.False(t, m1.Eq(m3))
	assert.False(t, m1.Eq(m4))
	assert.True(t, m1.Eq(m5))
}

func Test_ValueMap_Add(t *testing.T) {
	m1 := ValueMap{
		"A": NewFromFloat(3.0),
		"B": NewFromFloat(4.0),
	}

	m2 := ValueMap{
		"A": NewFromFloat(5.0),
		"B": NewFromFloat(6.0),
	}

	m3 := ValueMap{
		"A": NewFromFloat(8.0),
		"B": NewFromFloat(10.0),
	}

	m4 := ValueMap{"A": NewFromFloat(8.0)}

	assert.Equal(t, m3, m1.Add(m2))
	assert.Panics(t, func() { m1.Add(m4) })
}

func Test_ValueMap_AddScalar(t *testing.T) {
	x := NewFromFloat(5.0)

	m1 := ValueMap{
		"A": NewFromFloat(3.0),
		"B": NewFromFloat(4.0),
	}

	m2 := ValueMap{
		"A": NewFromFloat(3.0).Add(x),
		"B": NewFromFloat(4.0).Add(x),
	}

	assert.Equal(t, m2, m1.AddScalar(x))
}

func Test_ValueMap_DivScalar(t *testing.T) {
	x := NewFromFloat(5.0)

	m1 := ValueMap{
		"A": NewFromFloat(3.0),
		"B": NewFromFloat(4.0),
	}

	m2 := ValueMap{
		"A": NewFromFloat(3.0).Div(x),
		"B": NewFromFloat(4.0).Div(x),
	}

	assert.Equal(t, m2, m1.DivScalar(x))
}

func Test_ValueMap_Sum(t *testing.T) {
	m := ValueMap{
		"A": NewFromFloat(3.0),
		"B": NewFromFloat(4.0),
	}

	assert.Equal(t, NewFromFloat(7.0), m.Sum())
}

func Test_ValueMap_Normalize(t *testing.T) {
	a := NewFromFloat(3.0)
	b := NewFromFloat(4.0)
	c := a.Add(b)

	m := ValueMap{
		"A": a,
		"B": b,
	}

	n := ValueMap{
		"A": a.Div(c),
		"B": b.Div(c),
	}

	assert.True(t, m.Normalize().Eq(n))
}

func Test_ValueMap_Normalize_zero_sum(t *testing.T) {
	m := ValueMap{
		"A": Zero,
		"B": Zero,
	}

	assert.Panics(t, func() { m.Normalize() })
}
