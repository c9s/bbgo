package bitfinex

import (
	"testing"

	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
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

func Test_splitLocalSymbol(t *testing.T) {
	tests := []struct {
		input     string
		wantBase  string
		wantQuote string
	}{
		{"tBTCUSD", "BTC", "USD"},
		{"tETHUSD", "ETH", "USD"},
		{"tBTC:USD", "BTC", "USD"},
		{"tUSDTUSD", "UST", "USD"},
		{"tBTCUST", "BTC", "UST"},
		{"tMANAUSD", "MNA", "USD"},
		{"tWBTCUSD", "WBT", "USD"},
		{"tBTC", "BTC", ""},
		{"tBTC:UST", "BTC", "UST"},
		{"BTCUSD", "BTC", "USD"},
		{"BTC:USD", "BTC", "USD"},
		{"BTC", "BTC", ""},
	}

	for _, tt := range tests {
		base, quote := splitLocalSymbol(tt.input)
		if base != tt.wantBase || quote != tt.wantQuote {
			t.Errorf("splitLocalSymbol(%q) = (%q, %q), want (%q, %q)", tt.input, base, quote, tt.wantBase, tt.wantQuote)
		}
	}
}

func Test_convertBookEntries(t *testing.T) {
	entries := []bfxapi.BookEntry{
		{Price: fixedpoint.NewFromFloat(100.0), Amount: fixedpoint.NewFromFloat(2)}, // bid
		{Price: fixedpoint.NewFromFloat(101.0), Amount: fixedpoint.NewFromFloat(1)}, // bid
		{Price: fixedpoint.NewFromFloat(99.0), Amount: fixedpoint.NewFromFloat(-3)}, // ask
		{Price: fixedpoint.NewFromFloat(98.0), Amount: fixedpoint.NewFromFloat(-1)}, // ask
	}

	ob := convertBookEntries(entries, nil)

	if len(ob.Bids) != 2 {
		t.Errorf("expected 2 bids, got %d", len(ob.Bids))
	}
	if len(ob.Asks) != 2 {
		t.Errorf("expected 2 asks, got %d", len(ob.Asks))
	}

	// Bids should be sorted descending
	if ob.Bids[0].Price.Compare(ob.Bids[1].Price) < 0 {
		t.Errorf("bids not sorted descending: %v", ob.Bids)
	}
	// Asks should be sorted ascending
	if ob.Asks[0].Price.Compare(ob.Asks[1].Price) > 0 {
		t.Errorf("asks not sorted ascending: %v", ob.Asks)
	}
}
