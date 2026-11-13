package backpack

import (
	"sort"
	"strings"
	"sync"

	"github.com/c9s/bbgo/pkg/exchange/backpack/backpackapi"
	"github.com/c9s/bbgo/pkg/types"
)

// spotSymbolSyncMap and perpSymbolSyncMap map a global symbol to the local (native) symbol,
// e.g. "SOLUSDC" -> "SOL_USDC" and "SOLUSDC" -> "SOL_USDC_PERP".
//
// They are populated by syncLocalSymbolMap from the market list, so the live market data is
// always authoritative. Until that runs, toLocalSymbol falls back to splitting the global
// symbol on a known quote currency.
var (
	spotSymbolSyncMap sync.Map
	perpSymbolSyncMap sync.Map
)

// defaultQuoteCurrencies lists the quote currencies used to split a global symbol back into
// its base and quote parts.
//
// USDT comes first as the general default, but the list is deliberately not limited to it:
// exchanges also quote in USDC, USD and others (coinbase, for instance, lists both BTC/USD
// and BTC/USDC).
//
// Only the *length* ordering affects correctness, and sortedQuoteCurrencies enforces it:
// candidates of the same length can never shadow each other (BTCUSDT can only match USDT and
// BTCUSDC can only match USDC), but a shorter candidate must be tried last, otherwise
// "BTCUSDT" would be split into "BTC" + "USD" with a stray "T" left over.
var defaultQuoteCurrencies = []string{
	"USDT", "USDC", "BUSD", "TUSD", "USD", "EUR", "GBP", "BTC", "ETH",
}

// sortedQuoteCurrencies is defaultQuoteCurrencies ordered by descending length, computed once.
var sortedQuoteCurrencies = sortQuoteCurrenciesByLength(defaultQuoteCurrencies)

func sortQuoteCurrenciesByLength(quotes []string) []string {
	sorted := make([]string, len(quotes))
	copy(sorted, quotes)

	sort.SliceStable(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	return sorted
}

// splitGlobalSymbol splits a global symbol such as "BTCUSDT" into its base and quote parts by
// matching the longest known quote currency suffix.
//
// The quotes slice must be ordered by descending length; use sortQuoteCurrenciesByLength.
func splitGlobalSymbol(global string, quotes []string) (base, quote string, ok bool) {
	for _, q := range quotes {
		// the base part must be non-empty, otherwise "USDT" alone would split into "" + "USDT"
		if len(global) > len(q) && strings.HasSuffix(global, q) {
			return global[:len(global)-len(q)], q, true
		}
	}

	return "", "", false
}

// toGlobalSymbol converts a Backpack symbol into the bbgo global symbol.
//
// The perp suffix is dropped, so a spot market and its perpetual counterpart collapse onto the
// same global symbol (SOL_USDC and SOL_USDC_PERP both become SOLUSDC). This mirrors okex,
// where BTC-USDT and BTC-USDT-SWAP both become BTCUSDT; the two are told apart on the way back
// by the market type.
func toGlobalSymbol(local string) string {
	local = strings.TrimSuffix(local, perpSuffix)
	return strings.ReplaceAll(local, symbolSeparator, "")
}

// toLocalSymbol converts a bbgo global symbol into the Backpack symbol.
//
// It prefers the symbol map populated from the live market list, and falls back to splitting
// the symbol on a known quote currency. Callers on the Exchange should use
// Exchange.getLocalSymbol instead, which picks the market type from the futures settings.
func toLocalSymbol(global string, marketType ...backpackapi.MarketType) string {
	if len(marketType) == 1 && marketType[0] == backpackapi.MarketTypePerp {
		if s, ok := perpSymbolSyncMap.Load(global); ok {
			if localSymbol, ok := s.(string); ok {
				return localSymbol
			}
		}

		return buildLocalSymbol(global, true)
	}

	if s, ok := spotSymbolSyncMap.Load(global); ok {
		if localSymbol, ok := s.(string); ok {
			return localSymbol
		}
	}

	return buildLocalSymbol(global, false)
}

// buildLocalSymbol reconstructs a Backpack symbol from a global symbol without consulting the
// symbol map.
func buildLocalSymbol(global string, isPerp bool) string {
	base, quote, ok := splitGlobalSymbol(global, sortedQuoteCurrencies)
	if !ok {
		log.Warnf("unable to split the global symbol %q into a base and a quote currency, "+
			"using it as-is", global)
		return global
	}

	local := base + symbolSeparator + quote
	if isPerp {
		local += perpSuffix
	}

	return local
}

// syncLocalSymbolMap refreshes the local symbol map from the market list:
// existing symbols are updated, new ones are added and delisted ones are removed.
func syncLocalSymbolMap(markets types.MarketMap, marketType backpackapi.MarketType) {
	symbolMap := &spotSymbolSyncMap
	if marketType == backpackapi.MarketTypePerp {
		symbolMap = &perpSymbolSyncMap
	}

	existingSymbols := make(map[string]struct{}, len(markets))
	for symbol, market := range markets {
		symbolMap.Store(symbol, market.LocalSymbol)
		existingSymbols[symbol] = struct{}{}
	}

	symbolMap.Range(func(key, value interface{}) bool {
		symbol, ok := key.(string)
		if !ok {
			return true
		}

		if _, exists := existingSymbols[symbol]; !exists {
			symbolMap.Delete(symbol)
		}

		return true
	})
}

// isLocalSymbolOfMarketType reports whether a Backpack symbol belongs to the given market type.
//
// Several endpoints (the ticker list in particular) have no market type filter and return both
// the spot and the perpetual market. Since toGlobalSymbol drops the perp suffix, the two would
// collapse onto the same global symbol and overwrite each other, so the caller has to filter.
func isLocalSymbolOfMarketType(local string, marketType backpackapi.MarketType) bool {
	isPerp := strings.HasSuffix(local, perpSuffix)
	if marketType == backpackapi.MarketTypePerp {
		return isPerp
	}

	return !isPerp
}
