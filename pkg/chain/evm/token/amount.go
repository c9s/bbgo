// Package token provides ERC20 metadata lookup, balance queries and the
// conversion between raw on-chain base units and bbgo's fixedpoint.Value.
//
// Raw amounts are *big.Int and are the source of truth. fixedpoint.Value is a
// lossy projection of them: the default build is an int64 with 8 decimal
// places, so it can neither hold wei nor represent more than 8 decimals.
// Everything that talks to a contract must use the raw value.
package token

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var (
	// ErrPrecisionLoss is returned when a decimal string carries more
	// fractional digits than the token can represent.
	ErrPrecisionLoss = errors.New("amount has more decimals than the token supports")

	// ErrAmountOverflow is returned when a raw amount cannot be represented by
	// fixedpoint.Value in the current build.
	ErrAmountOverflow = errors.New("amount is out of the fixedpoint representable range")
)

// FormatRaw renders a raw base-unit amount as a decimal string using the given
// number of token decimals. It is exact and never overflows.
func FormatRaw(raw *big.Int, decimals int) string {
	if raw == nil {
		return "0"
	}

	if decimals <= 0 {
		return raw.String()
	}

	negative := raw.Sign() < 0
	digits := new(big.Int).Abs(raw).String()

	if len(digits) <= decimals {
		digits = strings.Repeat("0", decimals-len(digits)+1) + digits
	}

	integerPart := digits[:len(digits)-decimals]
	fractionPart := strings.TrimRight(digits[len(digits)-decimals:], "0")

	out := integerPart
	if fractionPart != "" {
		out += "." + fractionPart
	}

	if negative {
		out = "-" + out
	}

	return out
}

// ParseAmount converts a decimal string into a raw base-unit amount. It is
// exact: a string with more fractional digits than the token supports returns
// ErrPrecisionLoss instead of silently truncating. Use ParseAmountTruncated
// when truncation is intended.
func ParseAmount(s string, decimals int) (*big.Int, error) {
	raw, truncated, err := parseAmount(s, decimals)
	if err != nil {
		return nil, err
	}

	if truncated {
		return nil, fmt.Errorf("%w: %q with %d decimals", ErrPrecisionLoss, s, decimals)
	}

	return raw, nil
}

// ParseAmountTruncated converts a decimal string into a raw base-unit amount,
// truncating the extra fractional digits toward zero.
func ParseAmountTruncated(s string, decimals int) (*big.Int, error) {
	raw, _, err := parseAmount(s, decimals)
	return raw, err
}

func parseAmount(s string, decimals int) (raw *big.Int, truncated bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false, errors.New("amount is empty")
	}

	if decimals < 0 {
		return nil, false, fmt.Errorf("invalid token decimals %d", decimals)
	}

	negative := false
	switch s[0] {
	case '-':
		negative = true
		s = s[1:]
	case '+':
		s = s[1:]
	}

	integerPart, fractionPart, _ := strings.Cut(s, ".")
	if integerPart == "" {
		integerPart = "0"
	}

	if !isDigits(integerPart) || !isDigits(fractionPart) {
		return nil, false, fmt.Errorf("invalid decimal amount %q", s)
	}

	if len(fractionPart) > decimals {
		if strings.Trim(fractionPart[decimals:], "0") != "" {
			truncated = true
		}

		fractionPart = fractionPart[:decimals]
	}

	fractionPart += strings.Repeat("0", decimals-len(fractionPart))

	value, ok := new(big.Int).SetString(integerPart+fractionPart, 10)
	if !ok {
		return nil, false, fmt.Errorf("invalid decimal amount %q", s)
	}

	if negative {
		value.Neg(value)
	}

	return value, truncated, nil
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

// ToFixedPoint converts a raw base-unit amount into a fixedpoint.Value for use
// at the strategy and display boundary. The conversion is lossy beyond the
// precision of the current build, and it fails rather than wrap around when the
// integer part does not fit.
func ToFixedPoint(raw *big.Int, decimals int) (fixedpoint.Value, error) {
	s := FormatRaw(raw, decimals)

	v, err := fixedpoint.NewFromString(s)
	if err != nil {
		return fixedpoint.Zero, err
	}

	// detect the silent overflow of the int64-backed build by comparing the
	// integer parts before and after the conversion
	if integerPart(s) != integerPart(v.String()) {
		return fixedpoint.Zero, fmt.Errorf("%w: %s", ErrAmountOverflow, s)
	}

	return v, nil
}

// ToRaw converts a fixedpoint.Value into a raw base-unit amount. It returns
// ErrPrecisionLoss when the value carries more decimals than the token
// supports, so the caller rounds explicitly instead of losing dust silently.
func ToRaw(v fixedpoint.Value, decimals int) (*big.Int, error) {
	return ParseAmount(v.String(), decimals)
}

// ToRawTruncated converts a fixedpoint.Value into a raw base-unit amount,
// truncating the extra decimals toward zero.
func ToRawTruncated(v fixedpoint.Value, decimals int) (*big.Int, error) {
	return ParseAmountTruncated(v.String(), decimals)
}

func integerPart(s string) string {
	s = strings.TrimPrefix(s, "-")
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}

	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}

	return s
}
