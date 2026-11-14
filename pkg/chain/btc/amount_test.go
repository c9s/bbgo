package btc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAmount(t *testing.T) {
	tests := []struct {
		in   string
		want Amount
	}{
		{"0", 0},
		{"0.00000000", 0},
		{"0.00000001", 1},
		{"0.00000546", 546},
		{"3.12696278", 312696278},
		{"0.1776", 17760000},
		{"1", SatoshiPerBitcoin},
		{"21000000.00000000", MaxMoney},
		{"-0.5", -50000000},
		{"2.543e-05", 2543},
		{"1E-8", 1},
	}

	for _, tt := range tests {
		got, err := ParseAmount(tt.in)
		require.NoError(t, err, tt.in)
		assert.Equal(t, tt.want, got, tt.in)
	}
}

func TestParseAmountErrors(t *testing.T) {
	for _, in := range []string{
		"",
		"abc",
		"0.000000001", // below one satoshi
		"1e-9",
		"21000000.00000001", // above MaxMoney
		"1e9",
		"1.2.3",
		"0x10",
		"1/2",
	} {
		_, err := ParseAmount(in)
		assert.Error(t, err, in)
	}
}

func TestAmountJSON(t *testing.T) {
	var v struct {
		A Amount  `json:"a"`
		B *Amount `json:"b"`
	}

	// 0.1 + 0.2 is the classic float64 trap; the literals are parsed exactly
	require.NoError(t, json.Unmarshal([]byte(`{"a":0.10000000,"b":0.20000000}`), &v))
	assert.Equal(t, Amount(30000000), v.A+*v.B)

	out, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":0.10000000,"b":0.20000000}`, string(out))

	assert.Equal(t, "-0.00000546", Amount(-546).String())
	assert.Equal(t, "21000000.00000000", MaxMoney.String())
}
