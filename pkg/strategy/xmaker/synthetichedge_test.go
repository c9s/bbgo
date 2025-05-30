package xmaker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"

	"github.com/c9s/bbgo/pkg/types"
)

func TestSyntheticHedge_GetQuotePrices_BaseQuote(t *testing.T) {
	// source: BTCUSDT, fiat: USDTTWD, source quote == fiat base
	sourceMarket := types.Market{Symbol: "BTCUSDT", BaseCurrency: "BTC", QuoteCurrency: "USDT"}
	fiatMarket := types.Market{Symbol: "USDTTWD", BaseCurrency: "USDT", QuoteCurrency: "TWD"}

	sourceBook := types.NewStreamBook("BTCUSDT", "")
	sourceBook.Load(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: types.PriceVolumeSlice{
			{Price: Number(10000), Volume: Number(1)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(10010), Volume: Number(1)},
		},
	})

	fiatBook := types.NewStreamBook("USDTTWD", "")
	fiatBook.Load(types.SliceOrderBook{
		Symbol: "USDTTWD",
		Bids: types.PriceVolumeSlice{
			{Price: Number(30), Volume: Number(1000)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(31), Volume: Number(1000)},
		},
	})

	source := &TradingMarket{
		market:    sourceMarket,
		book:      sourceBook,
		depthBook: types.NewDepthBook(sourceBook, Number(1)),
	}
	fiat := &TradingMarket{
		market:    fiatMarket,
		book:      fiatBook,
		depthBook: types.NewDepthBook(fiatBook, Number(1)),
	}

	hedge := &SyntheticHedge{
		sourceMarket: source,
		fiatMarket:   fiat,
	}

	bid, ask, ok := hedge.GetQuotePrices()
	assert.True(t, ok)
	assert.Equal(t, Number(10000*30), bid)
	assert.Equal(t, Number(10010*31), ask)
}
