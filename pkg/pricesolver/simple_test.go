package pricesolver

import (
	"testing"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"

	"github.com/stretchr/testify/assert"
)

func TestSimplePriceResolver(t *testing.T) {
	markets := types.MarketMap{
		"BTCUSDT": types.Market{
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
		},
		"ETHUSDT": types.Market{
			BaseCurrency:  "ETH",
			QuoteCurrency: "USDT",
		},
		"BTCTWD": types.Market{
			BaseCurrency:  "BTC",
			QuoteCurrency: "TWD",
		},
		"ETHTWD": types.Market{
			BaseCurrency:  "ETH",
			QuoteCurrency: "TWD",
		},
		"USDTTWD": types.Market{
			BaseCurrency:  "USDT",
			QuoteCurrency: "TWD",
		},
		"ETHBTC": types.Market{
			BaseCurrency:  "ETH",
			QuoteCurrency: "BTC",
		},
	}

	t.Run("direct reference", func(t *testing.T) {
		pm := NewSimplePriceResolver(markets)
		pm.UpdateFromTrade(types.Trade{
			Symbol: "BTCUSDT",
			Price:  Number(48000.0),
		})
		pm.UpdateFromTrade(types.Trade{
			Symbol: "ETHUSDT",
			Price:  Number(2800.0),
		})
		pm.UpdateFromTrade(types.Trade{
			Symbol: "USDTTWD",
			Price:  Number(32.0),
		})

		finalPrice, ok := pm.ResolvePrice("BTC", "USDT")
		if assert.True(t, ok) {
			assert.Equal(t, "48000", finalPrice.String())
		}

		finalPrice, ok = pm.ResolvePrice("ETH", "USDT")
		if assert.True(t, ok) {
			assert.Equal(t, "2800", finalPrice.String())
		}

		finalPrice, ok = pm.ResolvePrice("USDT", "TWD")
		if assert.True(t, ok) {
			assert.Equal(t, "32", finalPrice.String())
		}
	})

	t.Run("simple reference", func(t *testing.T) {
		pm := NewSimplePriceResolver(markets)
		pm.UpdateFromTrade(types.Trade{
			Symbol: "BTCUSDT",
			Price:  Number(48000.0),
		})
		pm.UpdateFromTrade(types.Trade{
			Symbol: "ETHUSDT",
			Price:  Number(2800.0),
		})
		pm.UpdateFromTrade(types.Trade{
			Symbol: "USDTTWD",
			Price:  Number(32.0),
		})

		finalPrice, ok := pm.ResolvePrice("BTC", "TWD")
		if assert.True(t, ok) {
			assert.Equal(t, "1536000", finalPrice.String())
		}
	})

	t.Run("crypto reference", func(t *testing.T) {
		pm := NewSimplePriceResolver(markets)
		pm.UpdateFromTrade(types.Trade{
			Symbol: "BTCUSDT",
			Price:  Number(52000.0),
		})
		pm.UpdateFromTrade(types.Trade{
			Symbol: "ETHBTC",
			Price:  Number(0.055),
		})
		pm.UpdateFromTrade(types.Trade{
			Symbol: "USDTTWD",
			Price:  Number(32.0),
		})

		finalPrice, ok := pm.ResolvePrice("ETH", "USDT")
		if assert.True(t, ok) {
			assert.Equal(t, "2860", finalPrice.String())
		}
	})

	t.Run("inverse reference", func(t *testing.T) {
		pm := NewSimplePriceResolver(markets)
		pm.UpdateFromTrade(types.Trade{
			Symbol: "BTCTWD",
			Price:  Number(1536000.0),
		})
		pm.UpdateFromTrade(types.Trade{
			Symbol: "USDTTWD",
			Price:  Number(32.0),
		})

		finalPrice, ok := pm.ResolvePrice("BTC", "USDT")
		if assert.True(t, ok) {
			assert.Equal(t, "48000", finalPrice.String())
		}
	})

	t.Run("inverse reference", func(t *testing.T) {
		pm := NewSimplePriceResolver(markets)
		pm.UpdateFromTrade(types.Trade{
			Symbol: "BTCTWD",
			Price:  Number(1536000.0),
		})
		pm.UpdateFromTrade(types.Trade{
			Symbol: "USDTTWD",
			Price:  Number(32.0),
		})

		finalPrice, ok := pm.ResolvePrice("TWD", "USDT")
		if assert.True(t, ok) {
			assert.InDelta(t, 0.03125, finalPrice.Float64(), 0.0001)
		}
	})
}
