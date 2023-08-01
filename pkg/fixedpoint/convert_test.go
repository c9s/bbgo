package fixedpoint

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FormatString(t *testing.T) {
	cases := []struct {
		input    string
		prec     int
		expected string
	}{
		{input: "0.57", prec: 5, expected: "0.57000"},
		{input: "-0.57", prec: 5, expected: "-0.57000"},
		{input: "0.57123456", prec: 8, expected: "0.57123456"},
		{input: "-0.57123456", prec: 8, expected: "-0.57123456"},
		{input: "0.57123456", prec: 5, expected: "0.57123"},
		{input: "-0.57123456", prec: 5, expected: "-0.57123"},
		{input: "0.57123456", prec: 0, expected: "0"},
		{input: "-0.57123456", prec: 0, expected: "0"},
		{input: "0.57123456", prec: -1, expected: "0"},
		{input: "-0.57123456", prec: -1, expected: "0"},
		{input: "0.57123456", prec: -5, expected: "0"},
		{input: "-0.57123456", prec: -5, expected: "0"},
		{input: "0.57123456", prec: -9, expected: "0"},
		{input: "-0.57123456", prec: -9, expected: "0"},

		{input: "1.23456789", prec: 9, expected: "1.234567890"},
		{input: "-1.23456789", prec: 9, expected: "-1.234567890"},
		{input: "1.02345678", prec: 9, expected: "1.023456780"},
		{input: "-1.02345678", prec: 9, expected: "-1.023456780"},
		{input: "1.02345678", prec: 2, expected: "1.02"},
		{input: "-1.02345678", prec: 2, expected: "-1.02"},
		{input: "1.02345678", prec: 0, expected: "1"},
		{input: "-1.02345678", prec: 0, expected: "-1"},
		{input: "1.02345678", prec: -1, expected: "0"},
		{input: "-1.02345678", prec: -1, expected: "0"},
		{input: "1.02345678", prec: -10, expected: "0"},
		{input: "-1.02345678", prec: -10, expected: "0"},

		{input: "0.0001234", prec: 9, expected: "0.000123400"},
		{input: "-0.0001234", prec: 9, expected: "-0.000123400"},
		{input: "0.0001234", prec: 7, expected: "0.0001234"},
		{input: "-0.0001234", prec: 7, expected: "-0.0001234"},
		{input: "0.0001234", prec: 5, expected: "0.00012"},
		{input: "-0.0001234", prec: 5, expected: "-0.00012"},
		{input: "0.0001234", prec: 3, expected: "0.000"},
		{input: "-0.0001234", prec: 3, expected: "0.000"},
		{input: "0.0001234", prec: 2, expected: "0.00"},
		{input: "-0.0001234", prec: 2, expected: "0.00"},
		{input: "0.0001234", prec: 0, expected: "0"},
		{input: "-0.0001234", prec: 0, expected: "0"},
		{input: "0.00001234", prec: -1, expected: "0"},
		{input: "-0.00001234", prec: -1, expected: "0"},
		{input: "0.00001234", prec: -5, expected: "0"},
		{input: "-0.00001234", prec: -5, expected: "0"},
		{input: "0.00001234", prec: -9, expected: "0"},
		{input: "-0.00001234", prec: -9, expected: "0"},

		{input: "12.3456789", prec: 10, expected: "12.3456789000"},
		{input: "-12.3456789", prec: 10, expected: "-12.3456789000"},
		{input: "12.3456789", prec: 9, expected: "12.345678900"},
		{input: "-12.3456789", prec: 9, expected: "-12.345678900"},
		{input: "12.3456789", prec: 7, expected: "12.3456789"},
		{input: "-12.3456789", prec: 7, expected: "-12.3456789"},
		{input: "12.3456789", prec: 5, expected: "12.34567"},
		{input: "-12.3456789", prec: 5, expected: "-12.34567"},
		{input: "12.3456789", prec: 1, expected: "12.3"},
		{input: "-12.3456789", prec: 1, expected: "-12.3"},
		{input: "12.3456789", prec: 0, expected: "12"},
		{input: "-12.3456789", prec: 0, expected: "-12"},
		{input: "12.3456789", prec: -1, expected: "10"},
		{input: "-12.3456789", prec: -1, expected: "-10"},
		{input: "12.3456789", prec: -2, expected: "0"},
		{input: "-12.3456789", prec: -2, expected: "0"},
		{input: "12.3456789", prec: -3, expected: "0"},
		{input: "-12.3456789", prec: -3, expected: "0"},

		{input: "12345678.9", prec: 10, expected: "12345678.9000000000"},
		{input: "-12345678.9", prec: 10, expected: "-12345678.9000000000"},
		{input: "12345678.9", prec: 3, expected: "12345678.900"},
		{input: "-12345678.9", prec: 3, expected: "-12345678.900"},
		{input: "12345678.9", prec: 1, expected: "12345678.9"},
		{input: "-12345678.9", prec: 1, expected: "-12345678.9"},
		{input: "12345678.9", prec: 0, expected: "12345678"},
		{input: "-12345678.9", prec: 0, expected: "-12345678"},
		{input: "12345678.9", prec: -2, expected: "12345600"},
		{input: "-12345678.9", prec: -2, expected: "-12345600"},
		{input: "12345678.9", prec: -5, expected: "12300000"},
		{input: "-12345678.9", prec: -5, expected: "-12300000"},
		{input: "12345678.9", prec: -7, expected: "10000000"},
		{input: "-12345678.9", prec: -7, expected: "-10000000"},
		{input: "12345678.9", prec: -8, expected: "0"},
		{input: "-12345678.9", prec: -8, expected: "0"},
		{input: "12345678.9", prec: -10, expected: "0"},
		{input: "-12345678.9", prec: -10, expected: "0"},

		{input: "123000", prec: 7, expected: "123000.0000000"},
		{input: "-123000", prec: 7, expected: "-123000.0000000"},
		{input: "123000", prec: 2, expected: "123000.00"},
		{input: "-123000", prec: 2, expected: "-123000.00"},
		{input: "123000", prec: 0, expected: "123000"},
		{input: "-123000", prec: 0, expected: "-123000"},
		{input: "123000", prec: -1, expected: "123000"},
		{input: "-123000", prec: -1, expected: "-123000"},
		{input: "123000", prec: -5, expected: "100000"},
		{input: "-123000", prec: -5, expected: "-100000"},
		{input: "123000", prec: -6, expected: "0"},
		{input: "-123000", prec: -6, expected: "0"},
		{input: "123000", prec: -8, expected: "0"},
		{input: "-123000", prec: -8, expected: "0"},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%s with prec = %d, expected %s", c.input, c.prec, c.expected), func(t *testing.T) {
			v := MustNewFromString(c.input)
			s := v.FormatString(c.prec)
			assert.Equal(t, c.expected, s)
		})
	}
}
