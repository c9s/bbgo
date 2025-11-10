package hyperliquid

import "testing"

func TestToLocalSpotSymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        string
		stored     interface{}
		wantSymbol string
		wantIndex  int
	}{
		{name: "success", key: "UNITTEST_SUCCESS", stored: "@123", wantSymbol: "", wantIndex: 123},
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
