package fixedpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FormatString(t *testing.T) {
	assert := assert.New(t)

	t.Run("Value(57000000) with prec = 5, expected 0.57", func(t *testing.T) {
		v := Value(57000000)
		s := v.FormatString(5)
		assert.Equal("0.57000", s)
	})

	t.Run("Value(57123456) with prec = 5, expected 0.57123", func(t *testing.T) {
		v := Value(57123456)
		s := v.FormatString(5)
		assert.Equal("0.57123", s)
	})

	t.Run("Value(123456789) with prec = 9, expected 1.23456789", func(t *testing.T) {
		v := Value(123456789)
		s := v.FormatString(9)
		assert.Equal("1.234567890", s)
	})

	t.Run("Value(102345678) with prec = 9, expected 1.02345678", func(t *testing.T) {
		v := Value(102345678)
		s := v.FormatString(9)
		assert.Equal("1.023456780", s)
	})

	t.Run("Value(-57000000) with prec = 5, expected -0.57", func(t *testing.T) {
		v := Value(-57000000)
		s := v.FormatString(5)
		assert.Equal("-0.57000", s)
	})

	t.Run("Value(-123456789) with prec = 9, expected 1.23456789", func(t *testing.T) {
		v := Value(-123456789)
		s := v.FormatString(9)
		assert.Equal("-1.234567890", s)
	})

	t.Run("Value(1234567890) with prec = -1, expected 10", func(t *testing.T) {
		v := Value(1234567890)
		s := v.FormatString(-1)
		assert.Equal("10", s)
	})

	t.Run("Value(-1234) with prec = 3, expected = 0.000", func(t *testing.T) {
		v := Value(-1234)
		s := v.FormatString(3)
		assert.Equal("0.000", s)
	})
}
