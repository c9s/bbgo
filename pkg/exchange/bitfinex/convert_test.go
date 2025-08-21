package bitfinex

import (
	"testing"
)

func TestToLocalSymbol(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTCUSD", "tBTCUSD"},
		{"BTCUSDC", "tBTCUDC"},
		{"BTCUSDT", "tBTCUST"},
		{"WBTCUSDT", "tWBTUST"},
		{"BTCTUSD", "tBTCTSD"},
		{"USDTUSD", "tUSTUSD"},
		{"USDCUSD", "tUDCUSD"},
	}
	for _, test := range tests {
		result := toLocalSymbol(test.input)
		if result != test.expected {
			t.Errorf("toLocalSymbol(%q) = %q, want %q", test.input, result, test.expected)
		}
	}
}

func TestToGlobalSymbol(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"tBTCUSD", "BTCUSD"},
		{"tBTCUST", "BTCUSDT"},
	}
	for _, test := range tests {
		result := toGlobalSymbol(test.input)
		if result != test.expected {
			t.Errorf("toGlobalSymbol(%q) = %q, want %q", test.input, result, test.expected)
		}
	}
}

func TestToLocalCurrency(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"USDT", "UST"},
		{"USDC", "UDC"},
		{"TUSD", "TSD"},
		{"MANA", "MNA"},
		{"WBTC", "WBT"},
		{"BTC", "BTC"},         // not mapped, should return itself
		{"UNKNOWN", "UNKNOWN"}, // not mapped, should return itself
	}
	for _, test := range tests {
		result := toLocalCurrency(test.input)
		if result != test.expected {
			t.Errorf("toLocalCurrency(%q) = %q, want %q", test.input, result, test.expected)
		}
	}
}

func TestToGlobalCurrency(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"UDC", "USDC"},
		{"UST", "USDT"},
		{"TSD", "TUSD"},
		{"MNA", "MANA"},
		{"WBT", "WBTC"},
		{"BTC", "BTC"},         // not mapped, should return itself
		{"UNKNOWN", "UNKNOWN"}, // not mapped, should return itself
	}
	for _, test := range tests {
		result := toGlobalCurrency(test.input)
		if result != test.expected {
			t.Errorf("toGlobalCurrency(%q) = %q, want %q", test.input, result, test.expected)
		}
	}
}
