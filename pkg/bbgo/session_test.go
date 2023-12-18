package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_findPossibleMarketSymbols(t *testing.T) {
	t.Run("btcusdt", func(t *testing.T) {
		markets := types.MarketMap{
			"BTCUSDT": types.Market{},
			"BTCUSDC": types.Market{},
			"BTCUSD":  types.Market{},
			"BTCBUSD": types.Market{},
		}
		symbols := findPossibleMarketSymbols(markets, "BTC", "USDT")
		if assert.Len(t, symbols, 1) {
			assert.Equal(t, "BTCUSDT", symbols[0])
		}
	})

	t.Run("btcusd only", func(t *testing.T) {
		markets := types.MarketMap{
			"BTCUSD": types.Market{},
		}
		symbols := findPossibleMarketSymbols(markets, "BTC", "USDT")
		if assert.Len(t, symbols, 1) {
			assert.Equal(t, "BTCUSD", symbols[0])
		}
	})

	t.Run("usd to stable coin", func(t *testing.T) {
		markets := types.MarketMap{
			"BTCUSDT": types.Market{},
		}
		symbols := findPossibleMarketSymbols(markets, "BTC", "USD")
		if assert.Len(t, symbols, 1) {
			assert.Equal(t, "BTCUSDT", symbols[0])
		}
	})
}
