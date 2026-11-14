package btc

import (
	"bytes"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const (
	// SatoshiPerBitcoin is the number of satoshis in one BTC.
	SatoshiPerBitcoin = 100_000_000

	// MaxMoney is the consensus upper bound of any amount, 21 million BTC.
	MaxMoney Amount = 21_000_000 * SatoshiPerBitcoin
)

// Amount is a quantity of bitcoin in satoshis.
//
// The RPC writes amounts as JSON numbers in BTC with eight decimals
// ("value": 0.17777402). They are parsed from the literal, never through a
// float64, so 0.1 + 0.2 sums the same way the chain does. MaxMoney fits in an
// int64 with room to spare, unlike EVM wei, so no big.Int is needed.
type Amount int64

// ParseAmount parses a BTC decimal string such as "0.17777402" or "2.543e-05".
// More than eight decimals, which a satoshi cannot represent, is an error.
func ParseAmount(s string) (Amount, error) {
	if a, ok := parsePlainAmount(s); ok {
		return a, nil
	}

	// a JSON number with an exponent; big.Rat would also take fractions and
	// hex, which are not amounts
	if strings.Trim(s, "0123456789.eE+-") != "" {
		return 0, fmt.Errorf("btc: invalid amount %q", s)
	}

	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, fmt.Errorf("btc: invalid amount %q", s)
	}

	r.Mul(r, big.NewRat(SatoshiPerBitcoin, 1))
	if !r.IsInt() {
		return 0, fmt.Errorf("btc: amount %q has more than 8 decimals", s)
	}

	n := r.Num()
	if !n.IsInt64() || n.Int64() > int64(MaxMoney) || n.Int64() < -int64(MaxMoney) {
		return 0, fmt.Errorf("btc: amount %q out of range", s)
	}

	return Amount(n.Int64()), nil
}

// parsePlainAmount handles the [-]digits[.digits] form the RPC writes.
func parsePlainAmount(s string) (Amount, bool) {
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}

	whole, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		whole, frac = s[:i], s[i+1:]
	}

	if whole == "" || len(whole) > 8 || len(frac) > 8 || !isDigits(whole) || !isDigits(frac) {
		return 0, false
	}

	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, false
	}

	var f int64
	if frac != "" {
		if f, err = strconv.ParseInt(frac, 10, 64); err != nil {
			return 0, false
		}

		for i := len(frac); i < 8; i++ {
			f *= 10
		}
	}

	a := Amount(w*SatoshiPerBitcoin + f)
	if a > MaxMoney {
		return 0, false
	}

	if neg {
		a = -a
	}

	return a, true
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

// BTC returns the amount in BTC as a float64, for display only.
func (a Amount) BTC() float64 {
	return float64(a) / SatoshiPerBitcoin
}

// String formats the amount in BTC with all eight decimals, the way the RPC does.
func (a Amount) String() string {
	sign := ""
	v := int64(a)
	if v < 0 {
		sign = "-"
		v = -v
	}

	return fmt.Sprintf("%s%d.%08d", sign, v/SatoshiPerBitcoin, v%SatoshiPerBitcoin)
}

func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(a.String()), nil
}

func (a *Amount) UnmarshalJSON(data []byte) error {
	// the RPC writes numbers; a quoted amount is accepted for other sources
	data = bytes.Trim(data, `"`)
	if string(data) == "null" {
		return nil
	}

	parsed, err := ParseAmount(string(data))
	if err != nil {
		return err
	}

	*a = parsed
	return nil
}
