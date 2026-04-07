package hyperliquid

import (
	"testing"

	"github.com/c9s/bbgo/pkg/exchange/hyperliquid/hyperapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestToLocalSpotSymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        string
		stored     interface{}
		wantSymbol string
		wantIndex  int
	}{
		{name: "success", key: "UNITTEST_SUCCESS", stored: "BTC@123", wantSymbol: "BTC", wantIndex: 1123},
		{name: "invalid-format-missing-at", key: "UNITTEST_BADFMT", stored: "123", wantSymbol: "UNITTEST_BADFMT", wantIndex: -1},
		{name: "non-numeric", key: "UNITTEST_NAN", stored: "@abc", wantSymbol: "UNITTEST_NAN", wantIndex: -1},
		{name: "invalid-type", key: "UNITTEST_BADTYPE", stored: 123, wantSymbol: "UNITTEST_BADTYPE", wantIndex: -1},
		{name: "missing-key", key: "UNITTEST_MISSING", stored: nil, wantSymbol: "UNITTEST_MISSING", wantIndex: -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Setup
			spotSymbolSyncMap.Delete(tc.key)
			if tc.stored != nil {
				spotSymbolSyncMap.Store(tc.key, tc.stored)
			}
			t.Cleanup(func() { spotSymbolSyncMap.Delete(tc.key) })

			// Execute
			gotSymbol, gotIndex := toLocalSpotSymbol(tc.key)

			// Verify
			if gotSymbol != tc.wantSymbol || gotIndex != tc.wantIndex {
				t.Fatalf("toLocalSpotAsset(%q) = (%q, %d), want (%q, %d)", tc.key, gotSymbol, gotIndex, tc.wantSymbol, tc.wantIndex)
			}
		})
	}
}

func TestToLocalFuturesSymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        string
		stored     interface{}
		wantSymbol string
		wantIndex  int
	}{
		{name: "success", key: "FUT_SUCCESS", stored: "ETH@987", wantSymbol: "ETH", wantIndex: 987},
		{name: "invalid-format-missing-at", key: "FUT_BADFMT", stored: "987", wantSymbol: "FUT_BADFMT", wantIndex: -1},
		{name: "non-numeric", key: "FUT_NAN", stored: "@abc", wantSymbol: "FUT_NAN", wantIndex: -1},
		{name: "invalid-type", key: "FUT_BADTYPE", stored: 123, wantSymbol: "FUT_BADTYPE", wantIndex: -1},
		{name: "missing-key", key: "FUT_MISSING", stored: nil, wantSymbol: "FUT_MISSING", wantIndex: -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Setup
			futuresSymbolSyncMap.Delete(tc.key)
			if tc.stored != nil {
				futuresSymbolSyncMap.Store(tc.key, tc.stored)
			}
			t.Cleanup(func() { futuresSymbolSyncMap.Delete(tc.key) })

			// Execute
			gotSymbol, gotIndex := toLocalFuturesSymbol(tc.key)

			// Verify
			if gotSymbol != tc.wantSymbol || gotIndex != tc.wantIndex {
				t.Fatalf("toLocalFuturesAsset(%q) = (%q, %d), want (%q, %d)", tc.key, gotSymbol, gotIndex, tc.wantSymbol, tc.wantIndex)
			}
		})
	}
}

func TestToGlobalSymbol_Spot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        string
		stored     interface{}
		local      string
		wantSymbol string
	}{
		{name: "success", key: "SPOT_GLOBAL_SUCCESS", stored: "SOL@101", local: "SOL@101", wantSymbol: "SPOT_GLOBAL_SUCCESS"},
		{name: "success-coin-only", key: "SPOT_GLOBAL_COIN_ONLY", stored: "UNITTESTCOIN@66", local: "UNITTESTCOIN", wantSymbol: "SPOT_GLOBAL_COIN_ONLY"},
		{name: "invalid-value-type", key: "SPOT_GLOBAL_BADTYPE", stored: 101, local: "SOL@102", wantSymbol: "SOL@102"},
		{name: "missing-local-symbol", key: "SPOT_GLOBAL_MISSING", stored: "SOL@103", local: "SOL@999", wantSymbol: "SOL@999"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spotSymbolSyncMap.Delete(tc.key)
			spotSymbolSyncMap.Store(tc.key, tc.stored)
			t.Cleanup(func() { spotSymbolSyncMap.Delete(tc.key) })

			got := toGlobalSymbol(tc.local, false)
			if got != tc.wantSymbol {
				t.Fatalf("toGlobalSymbol(%q, false) = %q, want %q", tc.local, got, tc.wantSymbol)
			}
		})
	}
}

func TestToGlobalSymbol_Futures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        string
		stored     interface{}
		local      string
		wantSymbol string
	}{
		{name: "success", key: "FUT_GLOBAL_SUCCESS", stored: "BTC@11", local: "BTC@11", wantSymbol: "FUT_GLOBAL_SUCCESS"},
		{name: "success-coin-only", key: "FUT_GLOBAL_COIN_ONLY", stored: "UNITFUTCOIN@12", local: "UNITFUTCOIN", wantSymbol: "FUT_GLOBAL_COIN_ONLY"},
		{name: "invalid-value-type", key: "FUT_GLOBAL_BADTYPE", stored: 11, local: "BTC@12", wantSymbol: "BTC@12"},
		{name: "missing-local-symbol", key: "FUT_GLOBAL_MISSING", stored: "BTC@13", local: "BTC@999", wantSymbol: "BTC@999"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			futuresSymbolSyncMap.Delete(tc.key)
			futuresSymbolSyncMap.Store(tc.key, tc.stored)
			t.Cleanup(func() { futuresSymbolSyncMap.Delete(tc.key) })

			got := toGlobalSymbol(tc.local, true)
			if got != tc.wantSymbol {
				t.Fatalf("toGlobalSymbol(%q, true) = %q, want %q", tc.local, got, tc.wantSymbol)
			}
		})
	}
}

func TestToGlobalTimeInForce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want types.TimeInForce
	}{
		{name: "gtc", in: "Gtc", want: types.TimeInForceGTC},
		{name: "ioc", in: "Ioc", want: types.TimeInForceIOC},
		{name: "alo", in: "Alo", want: types.TimeInForceALO},
		{name: "empty fallback", in: "", want: types.TimeInForceGTC},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := toGlobalTimeInForce(tc.in)
			if got != tc.want {
				t.Fatalf("toGlobalTimeInForce(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestToGlobalOrderType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		order hyperapi.OpenOrder
		tif   types.TimeInForce
		want  types.OrderType
	}{
		{name: "limit", order: hyperapi.OpenOrder{OrderType: "Limit"}, tif: types.TimeInForceGTC, want: types.OrderTypeLimit},
		{name: "limit maker", order: hyperapi.OpenOrder{OrderType: "Limit"}, tif: types.TimeInForceALO, want: types.OrderTypeLimitMaker},
		{name: "market", order: hyperapi.OpenOrder{OrderType: "Market"}, tif: types.TimeInForceIOC, want: types.OrderTypeMarket},
		{name: "trigger take profit market", order: hyperapi.OpenOrder{IsTrigger: true, OrderType: "Market", TriggerCondition: "tp"}, tif: types.TimeInForceIOC, want: types.OrderTypeTakeProfitMarket},
		{name: "trigger stop limit", order: hyperapi.OpenOrder{IsTrigger: true, OrderType: "Limit", TriggerCondition: "sl"}, tif: types.TimeInForceGTC, want: types.OrderTypeStopLimit},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := toGlobalOrderType(tc.order, tc.tif)
			if got != tc.want {
				t.Fatalf("toGlobalOrderType(%+v, %q) = %q, want %q", tc.order, tc.tif, got, tc.want)
			}
		})
	}
}

func TestToGlobalOrder(t *testing.T) {
	t.Parallel()

	order := hyperapi.OpenOrder{
		Coin:      "BTC",
		LimitPx:   fixedpoint.NewFromFloat(100.0),
		Sz:        fixedpoint.NewFromFloat(0.5),
		Side:      "B",
		OrderType: "Limit",
		Tif:       "Alo",
		Oid:       12345,
	}

	got := toGlobalOrder(order, true)
	if got.Symbol != "BTCUSDC" {
		t.Fatalf("Symbol = %s, want BTCUSDC", got.Symbol)
	}
	if got.Type != types.OrderTypeLimitMaker {
		t.Fatalf("Type = %s, want %s", got.Type, types.OrderTypeLimitMaker)
	}
	if got.TimeInForce != types.TimeInForceALO {
		t.Fatalf("TimeInForce = %s, want %s", got.TimeInForce, types.TimeInForceALO)
	}
}
