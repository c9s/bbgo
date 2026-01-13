//go:build !dnum

package xmaker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

// TestSplitHedge_BalanceWeightedQuote verifies the balance-weighted bid/ask aggregation across hedge markets.
func TestSplitHedge_BalanceWeightedQuote(t *testing.T) {
	ctx := context.Background()

	// Create two markets (same symbol) with distinct sessions/streams
	market := Market("BTCUSDT")

	ctrl1 := gomock.NewController(t)
	defer ctrl1.Finish()
	s1, md1, _ := newMockSession(ctrl1, ctx, market.Symbol)

	ctrl2 := gomock.NewController(t)
	defer ctrl2.Finish()
	s2, md2, _ := newMockSession(ctrl2, ctx, market.Symbol)

	// Override balances to match the example:
	// s1: 10 BTC, 10000 USDT; s2: 1 BTC, 200 USDT
	s1.Account.UpdateBalances(types.BalanceMap{
		"BTC":  types.NewBalance("BTC", Number(10)),
		"USDT": types.NewBalance("USDT", Number(10000)),
	})
	s2.Account.UpdateBalances(types.BalanceMap{
		"BTC":  types.NewBalance("BTC", Number(1)),
		"USDT": types.NewBalance("USDT", Number(200)),
	})

	// Create hedge markets and connect streams
	hm1 := NewHedgeMarket(&HedgeMarketConfig{SymbolSelector: market.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: Number(1)}, s1, market)
	hm2 := NewHedgeMarket(&HedgeMarketConfig{SymbolSelector: market.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: Number(1)}, s2, market)

	assert.NoError(t, hm1.stream.Connect(ctx))
	assert.NoError(t, hm2.stream.Connect(ctx))

	// Provide order books; use slightly different prices per session
	md1.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: market.Symbol,
		Bids:   types.PriceVolumeSlice{{Price: Number(10000), Volume: Number(100)}},
		Asks:   types.PriceVolumeSlice{{Price: Number(10010), Volume: Number(100)}},
	})
	md2.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: market.Symbol,
		Bids:   types.PriceVolumeSlice{{Price: Number(9990), Volume: Number(100)}},
		Asks:   types.PriceVolumeSlice{{Price: Number(10020), Volume: Number(100)}},
	})

	// Assemble SplitHedge with these instances
	sh := &SplitHedge{
		Enabled:              true,
		hedgeMarketInstances: map[string]*HedgeMarket{"binance": hm1, "bitfinex": hm2},
	}

	// Compute expected weighted prices using the helper methods and balances
	b1, a1 := hm1.GetQuotePriceBySessionBalances()
	b2, a2 := hm2.GetQuotePriceBySessionBalances()
	base1, quote1 := hm1.GetBaseQuoteAvailableBalances()
	base2, quote2 := hm2.GetBaseQuoteAvailableBalances()

	// Weighted ask by base balances
	expectedAsk := a1.Mul(base1).Add(a2.Mul(base2)).Div(base1.Add(base2))
	// Weighted bid by quote balances
	expectedBid := b1.Mul(quote1).Add(b2.Mul(quote2)).Div(quote1.Add(quote2))

	bid, ask, _ := sh.GetBalanceWeightedQuotePrice()

	assert.InEpsilon(t, expectedAsk.Float64(), ask.Float64(), 1e-9)
	assert.InEpsilon(t, expectedBid.Float64(), bid.Float64(), 1e-9)
}

// TestSplitHedge_BalanceWeightedQuote_ZeroWeights ensures zero-weight side returns zero price.
func TestSplitHedge_BalanceWeightedQuote_ZeroWeights(t *testing.T) {
	ctx := context.Background()
	market := Market("BTCUSDT")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	s, md, _ := newMockSession(ctrl, ctx, market.Symbol)

	// Zero base and quote balances
	s.Account.UpdateBalances(types.BalanceMap{
		"BTC":  types.NewBalance("BTC", Number(0)),
		"USDT": types.NewBalance("USDT", Number(0)),
	})

	hm := NewHedgeMarket(&HedgeMarketConfig{SymbolSelector: market.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: Number(1)}, s, market)
	assert.NoError(t, hm.stream.Connect(ctx))

	md.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: market.Symbol,
		Bids:   types.PriceVolumeSlice{{Price: Number(10000), Volume: Number(100)}},
		Asks:   types.PriceVolumeSlice{{Price: Number(10010), Volume: Number(100)}},
	})

	sh := &SplitHedge{Enabled: true, hedgeMarketInstances: map[string]*HedgeMarket{"m": hm}}

	bid, ask, _ := sh.GetBalanceWeightedQuotePrice()
	assert.True(t, bid.IsZero())
	assert.True(t, ask.IsZero())
}
