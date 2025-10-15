package hyperliquid

import "testing"

func TestToLocalSpotAsset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		key    string
		stored interface{}
		want   string
	}{
		{name: "success", key: "UNITTEST_SUCCESS", stored: "@123", want: "1123"},
		{name: "invalid-format-missing-at", key: "UNITTEST_BADFMT", stored: "123", want: "UNITTEST_BADFMT"},
		{name: "non-numeric", key: "UNITTEST_NAN", stored: "@abc", want: "UNITTEST_NAN"},
		{name: "invalid-type", key: "UNITTEST_BADTYPE", stored: 123, want: "UNITTEST_BADTYPE"},
		{name: "missing-key", key: "UNITTEST_MISSING", stored: nil, want: "UNITTEST_MISSING"},
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
			got := toLocalSpotAsset(tc.key)

			// Verify
			if got != tc.want {
				t.Fatalf("toLocalSpotAsset(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestToLocalFuturesAsset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		key    string
		stored interface{}
		want   string
	}{
		{name: "success", key: "FUT_SUCCESS", stored: "ETH@987", want: "987"},
		{name: "invalid-format-missing-at", key: "FUT_BADFMT", stored: "987", want: "FUT_BADFMT"},
		{name: "non-numeric", key: "FUT_NAN", stored: "@abc", want: "abc"},
		{name: "invalid-type", key: "FUT_BADTYPE", stored: 123, want: "FUT_BADTYPE"},
		{name: "missing-key", key: "FUT_MISSING", stored: nil, want: "FUT_MISSING"},
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
			got := toLocalFuturesAsset(tc.key)

			// Verify
			if got != tc.want {
				t.Fatalf("toLocalFuturesAsset(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}
