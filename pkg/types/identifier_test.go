package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateIdentifier(t *testing.T) {
	valid := []string{
		"binance",
		"binance_margin",
		"binance_margin_linkusdt",
		"max2",
		"2fast",
		"_",
		"a",
		strings.Repeat("a", MaxIdentifierLength),
	}
	for _, s := range valid {
		t.Run("valid/"+s, func(t *testing.T) {
			assert.NoError(t, ValidateIdentifier(s))
		})
	}

	invalid := map[string]struct {
		in   string
		want string
	}{
		"empty":            {"", "is empty"},
		"uppercase":        {"Binance", `'B' at position 0`},
		"uppercase inside": {"binanceMargin", `'M' at position 7`},
		"hyphen":           {"binance-margin", `'-' at position 7`},
		"space":            {"binance margin", `' ' at position 7`},
		"leading space":    {" binance", `' ' at position 0`},
		"trailing newline": {"binance\n", `'\n' at position 7`},
		"dot":              {"binance.margin", `'.' at position 7`},
		"slash":            {"binance/margin", `'/' at position 7`},
		"quote":            {"binance'", `'\'' at position 7`},
		"non-ASCII letter": {"binancé", `'é' at position 6`},
		"non-ASCII digit":  {"binance１", `'１' at position 7`},
		"CJK":              {"帳戶", `'帳' at position 0`},
		"too long":         {strings.Repeat("a", MaxIdentifierLength+1), "has 65 characters, expected at most 64"},
	}
	for name, c := range invalid {
		t.Run("invalid/"+name, func(t *testing.T) {
			err := ValidateIdentifier(c.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

// A session added without a config is named after its exchange, so every
// exchange name has to be an identifier.
func TestSupportedExchangeNamesAreIdentifiers(t *testing.T) {
	for name := range SupportedExchanges {
		assert.NoError(t, ValidateIdentifier(name.String()), name)
	}
}
