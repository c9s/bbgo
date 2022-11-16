//go:build dnum

package fixedpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDelta(t *testing.T) {
	f1 := MustNewFromString("0.0009763593380614657")
	f2 := NewFromInt(42300)
	assert.InDelta(t, f1.Mul(f2).Float64(), 41.3, 1e-14)
}

func TestFloor(t *testing.T) {
	f1 := MustNewFromString("10.333333")
	f2 := f1.Floor()
	assert.Equal(t, "10", f2.String())
}

func TestInternal(t *testing.T) {
	r := &reader{"1.1e-15", 0}
	c, e := r.getCoef()
	assert.Equal(t, uint64(1100000000000000), c)
	assert.Equal(t, 1, e)
	f := MustNewFromString("1.1e-15")
	digits := getDigits(f.coef)
	assert.Equal(t, "11", digits)
	f = MustNewFromString("1.00000000000000111")
	assert.Equal(t, "1.000000000000001", f.String())
	f = MustNewFromString("1.1e-15")
	assert.Equal(t, "0.0000000000000011", f.String())
	assert.Equal(t, 16, f.NumFractionalDigits())
	f = MustNewFromString("1.00000000000000111")
	assert.Equal(t, "1.000000000000001", f.String())
	f = MustNewFromString("0.00000000000000000001000111")
	assert.Equal(t, "0.00000000000000000001000111", f.String())
	f = MustNewFromString("0.000000000000000000001000111")
	assert.Equal(t, "1.000111e-21", f.String())
	f = MustNewFromString("1e-100")
	assert.Equal(t, 100, f.NumFractionalDigits())
}
