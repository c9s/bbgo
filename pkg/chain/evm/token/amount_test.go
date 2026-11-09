package token

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func bigFromString(t *testing.T, s string) *big.Int {
	t.Helper()
	v, ok := new(big.Int).SetString(s, 10)
	assert.True(t, ok, "invalid big int %q", s)
	return v
}

func TestFormatRaw(t *testing.T) {
	assert.Equal(t, "0", FormatRaw(nil, 18))
	assert.Equal(t, "0", FormatRaw(big.NewInt(0), 18))
	assert.Equal(t, "1", FormatRaw(bigFromString(t, "1000000000000000000"), 18))
	assert.Equal(t, "1.5", FormatRaw(bigFromString(t, "1500000000000000000"), 18))
	assert.Equal(t, "0.000000000000000001", FormatRaw(big.NewInt(1), 18))
	assert.Equal(t, "-2.5", FormatRaw(bigFromString(t, "-2500000000000000000"), 18))

	// USDT is 6 decimals on Ethereum and 18 decimals on BSC: the same raw value
	// is a completely different amount
	raw := bigFromString(t, "1000000")
	assert.Equal(t, "1", FormatRaw(raw, 6))
	assert.Equal(t, "0.000000000001", FormatRaw(raw, 18))

	// WBTC is 8 decimals
	assert.Equal(t, "0.00000001", FormatRaw(big.NewInt(1), 8))

	// a zero-decimal token
	assert.Equal(t, "42", FormatRaw(big.NewInt(42), 0))
}

func TestParseAmount(t *testing.T) {
	raw, err := ParseAmount("1.5", 18)
	assert.NoError(t, err)
	assert.Equal(t, bigFromString(t, "1500000000000000000"), raw)

	raw, err = ParseAmount("0.000001", 6)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(1), raw)

	raw, err = ParseAmount("-1", 8)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(-100000000), raw)

	raw, err = ParseAmount(".5", 2)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(50), raw)

	// trailing zeros beyond the token decimals are not a precision loss
	raw, err = ParseAmount("1.500", 2)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(150), raw)

	_, err = ParseAmount("1.2345678", 6)
	assert.ErrorIs(t, err, ErrPrecisionLoss)

	_, err = ParseAmount("abc", 18)
	assert.Error(t, err)

	_, err = ParseAmount("", 18)
	assert.Error(t, err)
}

func TestParseAmountTruncated(t *testing.T) {
	raw, err := ParseAmountTruncated("1.2345678", 6)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(1234567), raw)
}

func TestAmountRoundTrip(t *testing.T) {
	for _, decimals := range []int{6, 8, 18} {
		for _, s := range []string{"0", "1", "123.456", "0.000001"} {
			raw, err := ParseAmountTruncated(s, decimals)
			if !assert.NoError(t, err) {
				continue
			}

			formatted := FormatRaw(raw, decimals)
			reparsed, err := ParseAmountTruncated(formatted, decimals)
			assert.NoError(t, err)
			assert.Equal(t, raw, reparsed, "decimals=%d input=%s", decimals, s)
		}
	}
}

func TestToFixedPoint(t *testing.T) {
	v, err := ToFixedPoint(bigFromString(t, "1500000000000000000"), 18)
	assert.NoError(t, err)
	assert.Equal(t, fixedpoint.NewFromFloat(1.5), v)

	// 1 wei is below the representable precision and rounds to zero rather
	// than producing a wrong number
	v, err = ToFixedPoint(big.NewInt(1), 18)
	assert.NoError(t, err)
	assert.True(t, v.IsZero())

	// the int64-backed build cannot hold this, and must not wrap around
	_, err = ToFixedPoint(bigFromString(t, "100000000000000000000000000000"), 8)
	assert.Error(t, err)
}

func TestToRaw(t *testing.T) {
	raw, err := ToRaw(fixedpoint.NewFromFloat(1.5), 18)
	assert.NoError(t, err)
	assert.Equal(t, bigFromString(t, "1500000000000000000"), raw)

	raw, err = ToRaw(fixedpoint.NewFromFloat(2), 6)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(2000000), raw)
}
