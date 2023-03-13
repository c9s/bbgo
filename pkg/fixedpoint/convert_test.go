package fixedpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FormatString(t *testing.T) {
	assert := assert.New(t)

	t.Run("0.57 with prec = 5, expected 0.57", func(t *testing.T) {
		v := MustNewFromString("0.57")
		s := v.FormatString(5)
		assert.Equal("0.57000", s)
	})

	t.Run("0.57123456 with prec = 5, expected 0.57123", func(t *testing.T) {
		v := MustNewFromString("0.57123456")
		s := v.FormatString(5)
		assert.Equal("0.57123", s)
	})

	t.Run("1.23456789 with prec = 9, expected 1.23456789", func(t *testing.T) {
		v := MustNewFromString("1.23456789")
		s := v.FormatString(9)
		assert.Equal("1.234567890", s)
	})

	t.Run("1.02345678 with prec = 9, expected 1.02345678", func(t *testing.T) {
		v := MustNewFromString("1.02345678")
		s := v.FormatString(9)
		assert.Equal("1.023456780", s)
	})

	t.Run("-0.57 with prec = 5, expected -0.57", func(t *testing.T) {
		v := MustNewFromString("-0.57")
		s := v.FormatString(5)
		assert.Equal("-0.57000", s)
	})

	t.Run("-1.23456789 with prec = 9, expected 1.23456789", func(t *testing.T) {
		v := MustNewFromString("-1.23456789")
		s := v.FormatString(9)
		assert.Equal("-1.234567890", s)
	})

	t.Run("-0.00001234 with prec = 3, expected = 0.000", func(t *testing.T) {
		v := MustNewFromString("-0.0001234")
		s := v.FormatString(3)
		assert.Equal("0.000", s)
	})

	// comment out negative precision for dnum testing
	/*
		t.Run("12.3456789 with prec = -1, expected 10", func(t *testing.T) {
			v := MustNewFromString("12.3456789")
			s := v.FormatString(-1)
			assert.Equal("10", s)
		})

		t.Run("12.3456789 with prec = -3, expected = 0", func(t *testing.T) {
			v := MustNewFromString("12.3456789")
			s := v.FormatString(-2)
			assert.Equal("0", s)
		})
	*/
}
