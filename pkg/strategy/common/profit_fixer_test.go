package common

import (
	"context"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

// mockFixer implements the iFixer interface for testing

func newMockFixer() *ProfitFixer {
	fixer := &ProfitFixer{
		Environment: &bbgo.Environment{},
	}
	fixer.queryTrades = func(ctx context.Context, symbol string, since, until time.Time) ([]types.Trade, error) {
		return []types.Trade{}, nil
	}
	return fixer
}

func TestProfitFixerConfigEqual(t *testing.T) {
	t.Run(
		"BothEmpty", func(t *testing.T) {
			c1 := ProfitFixerConfig{}
			c2 := ProfitFixerConfig{}
			assert.True(t, c1.Equal(c2))
		},
	)
	t.Run(
		"DifferentTradeSince", func(t *testing.T) {
			c1 := ProfitFixerConfig{TradesSince: types.Time(time.Now())}
			c2 := ProfitFixerConfig{TradesSince: types.Time(time.Now().Add(time.Hour))}
			assert.False(t, c1.Equal(c2))
		},
	)
	t.Run(
		"SameTradeSince", func(t *testing.T) {
			now := time.Now()
			c1 := ProfitFixerConfig{TradesSince: types.Time(now)}
			c2 := ProfitFixerConfig{TradesSince: types.Time(now)}
			assert.True(t, c1.Equal(c2))
		},
	)
	t.Run(
		"DifferentPatch", func(t *testing.T) {
			c1 := ProfitFixerConfig{Patch: "a"}
			c2 := ProfitFixerConfig{Patch: "b"}
			assert.False(t, c1.Equal(c2))
		},
	)
	t.Run(
		"SamePatch", func(t *testing.T) {
			c1 := ProfitFixerConfig{Patch: "a"}
			c2 := ProfitFixerConfig{Patch: "a"}
			assert.True(t, c1.Equal(c2))
		},
	)
	t.Run(
		"AllSame", func(t *testing.T) {
			now := time.Now()
			c1 := ProfitFixerConfig{TradesSince: types.Time(now), Patch: "a"}
			c2 := ProfitFixerConfig{TradesSince: types.Time(now), Patch: "a"}
			assert.True(t, c1.Equal(c2))
		},
	)
}

func Test_fixFromTrades(t *testing.T) {
	symbol := "BTCUSDT"
	market := types.Market{
		Symbol:          symbol,
		BaseCurrency:    "BTC",
		QuoteCurrency:   "USDT",
		MinNotional:     fixedpoint.MustNewFromString("0.001"),
		VolumePrecision: 8,
		PricePrecision:  2,
	}

	t.Run("empty trades", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		err := fixer.fixFromTrades([]types.Trade{}, nil, stats, position)

		assert.NoError(t, err)
		assert.True(t, position.Base.IsZero())
		assert.True(t, stats.AccumulatedPnL.IsZero())
	})

	t.Run("single buy trade without profit", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		trades := []types.Trade{
			{
				ID:            1,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(10000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(10000),
				Fee:           fixedpoint.NewFromFloat(0.1),
				FeeCurrency:   "BNB",
				Time:          types.Time(time.Now()),
			},
		}

		err := fixer.fixFromTrades(trades, nil, stats, position)

		assert.NoError(t, err)
		assert.Equal(t, fixedpoint.NewFromInt(1).String(), position.Base.String())
		assert.True(t, stats.AccumulatedPnL.IsZero())
	})

	t.Run("buy and sell trades with profit", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		now := time.Now()
		trades := []types.Trade{
			{
				ID:            1,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(10000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(10000),
				Fee:           fixedpoint.NewFromFloat(10),
				FeeCurrency:   "USDT",
				Time:          types.Time(now),
			},
			{
				ID:            2,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeSell,
				Price:         fixedpoint.NewFromInt(11000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(11000),
				Fee:           fixedpoint.NewFromFloat(11),
				FeeCurrency:   "USDT",
				Time:          types.Time(now.Add(time.Hour)),
			},
		}

		err := fixer.fixFromTrades(trades, nil, stats, position)

		assert.NoError(t, err)
		assert.True(t, position.Base.IsZero())
		// Fee is deducted from quoteQuantity, so:
		// Buy: avgCost = (10000-10)/1 = 9990
		// Sell: profit = (11000-9990)*1 = 1010
		assert.Equal(t, fixedpoint.NewFromInt(1010).String(), stats.AccumulatedPnL.String())
	})

	t.Run("multiple trades with partial closes", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		now := time.Now()
		trades := []types.Trade{
			{
				ID:            1,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(10000),
				Quantity:      fixedpoint.NewFromInt(2),
				QuoteQuantity: fixedpoint.NewFromInt(20000),
				Fee:           fixedpoint.NewFromFloat(20),
				FeeCurrency:   "USDT",
				Time:          types.Time(now),
			},
			{
				ID:            2,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeSell,
				Price:         fixedpoint.NewFromInt(11000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(11000),
				Fee:           fixedpoint.NewFromFloat(11),
				FeeCurrency:   "USDT",
				Time:          types.Time(now.Add(time.Hour)),
			},
			{
				ID:            3,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeSell,
				Price:         fixedpoint.NewFromInt(12000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(12000),
				Fee:           fixedpoint.NewFromFloat(12),
				FeeCurrency:   "USDT",
				Time:          types.Time(now.Add(2 * time.Hour)),
			},
		}

		err := fixer.fixFromTrades(trades, nil, stats, position)

		assert.NoError(t, err)
		assert.True(t, position.Base.IsZero())
		// Buy: avgCost = (20000-20)/2 = 9990
		// First sell: profit = (11000-9990)*1 = 1010
		// Second sell: profit = (12000-9990)*1 = 2010
		// Total: 1010 + 2010 = 3020
		assert.Equal(t, fixedpoint.NewFromInt(3020).String(), stats.AccumulatedPnL.String())
	})

	t.Run("short position trades", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		now := time.Now()
		trades := []types.Trade{
			{
				ID:            1,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeSell,
				Price:         fixedpoint.NewFromInt(10000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(10000),
				Fee:           fixedpoint.NewFromFloat(10),
				FeeCurrency:   "USDT",
				Time:          types.Time(now),
			},
			{
				ID:            2,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(9000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(9000),
				Fee:           fixedpoint.NewFromFloat(9),
				FeeCurrency:   "USDT",
				Time:          types.Time(now.Add(time.Hour)),
			},
		}

		err := fixer.fixFromTrades(trades, nil, stats, position)

		assert.NoError(t, err)
		assert.True(t, position.Base.IsZero())
		// Short sell: avgCost = (10000+10)/1 = 10010
		// Buy back: profit = (10010-9000)*1 = 1010, but fee was deducted so avgCost is actually (10000-10)/1=9990
		// Actually for short: avgCost = (10000-10)/1 = 9990, profit = (9990-9000)*1 = 990
		assert.Equal(t, fixedpoint.NewFromInt(990).String(), stats.AccumulatedPnL.String())
	})

	t.Run("with token fee prices", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		now := time.Now()
		dateStr := now.Format(time.DateOnly)

		tokenFeePrices := map[tokenFeeKey]fixedpoint.Value{
			{
				token:        "BNB",
				exchangeName: types.ExchangeBinance,
				date:         dateStr,
			}: fixedpoint.NewFromInt(500),
		}

		trades := []types.Trade{
			{
				ID:            1,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(10000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(10000),
				Fee:           fixedpoint.NewFromFloat(0.02), // 0.02 BNB
				FeeCurrency:   "BNB",
				Time:          types.Time(now),
			},
			{
				ID:            2,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeSell,
				Price:         fixedpoint.NewFromInt(11000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(11000),
				Fee:           fixedpoint.NewFromFloat(0.02), // 0.02 BNB
				FeeCurrency:   "BNB",
				Time:          types.Time(now.Add(time.Hour)),
			},
		}

		err := fixer.fixFromTrades(trades, tokenFeePrices, stats, position)

		assert.NoError(t, err)
		assert.True(t, position.Base.IsZero())
		// BNB fee converted to USDT: 0.02 * 500 = 10 per trade
		// Buy: avgCost = (10000 + 10)/1 = 10010
		// Sell: profit = (11000 - 10010)*1 = 990 (AccumulatedPnL tracks gross profit)
		assert.Equal(t, fixedpoint.NewFromInt(990).String(), stats.AccumulatedPnL.String())
	})

	t.Run("base currency fee", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		now := time.Now()
		trades := []types.Trade{
			{
				ID:            1,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(10000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(10000),
				Fee:           fixedpoint.NewFromFloat(0.001), // 0.001 BTC fee
				FeeCurrency:   "BTC",
				Time:          types.Time(now),
			},
			{
				ID:            2,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeSell,
				Price:         fixedpoint.NewFromInt(11000),
				Quantity:      fixedpoint.NewFromFloat(0.999), // selling the remaining after fee
				QuoteQuantity: fixedpoint.NewFromFloat(10989),
				Fee:           fixedpoint.NewFromFloat(11),
				FeeCurrency:   "USDT",
				Time:          types.Time(now.Add(time.Hour)),
			},
		}

		err := fixer.fixFromTrades(trades, nil, stats, position)

		assert.NoError(t, err)
		assert.True(t, position.Base.IsZero())
		// Position should handle base currency fee correctly
		assert.False(t, stats.AccumulatedPnL.IsZero())
	})

	t.Run("no profit made - only opening position", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		now := time.Now()
		trades := []types.Trade{
			{
				ID:            1,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(10000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(10000),
				Fee:           fixedpoint.NewFromFloat(10),
				FeeCurrency:   "USDT",
				Time:          types.Time(now),
			},
			{
				ID:            2,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(11000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(11000),
				Fee:           fixedpoint.NewFromFloat(11),
				FeeCurrency:   "USDT",
				Time:          types.Time(now.Add(time.Hour)),
			},
		}

		err := fixer.fixFromTrades(trades, nil, stats, position)

		assert.NoError(t, err)
		assert.Equal(t, fixedpoint.NewFromInt(2).String(), position.Base.String())
		assert.True(t, stats.AccumulatedPnL.IsZero()) // No profit made, position still open
	})

	t.Run("mixed fee currencies", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		now := time.Now()
		dateStr := now.Format(time.DateOnly)

		tokenFeePrices := map[tokenFeeKey]fixedpoint.Value{
			{
				token:        "BNB",
				exchangeName: types.ExchangeBinance,
				date:         dateStr,
			}: fixedpoint.NewFromInt(500),
		}

		trades := []types.Trade{
			{
				ID:            1,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(10000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(10000),
				Fee:           fixedpoint.NewFromFloat(0.02), // BNB fee
				FeeCurrency:   "BNB",
				Time:          types.Time(now),
			},
			{
				ID:            2,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeSell,
				Price:         fixedpoint.NewFromInt(11000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(11000),
				Fee:           fixedpoint.NewFromFloat(11), // USDT fee
				FeeCurrency:   "USDT",
				Time:          types.Time(now.Add(time.Hour)),
			},
		}

		err := fixer.fixFromTrades(trades, tokenFeePrices, stats, position)

		assert.NoError(t, err)
		assert.True(t, position.Base.IsZero())
		// Buy with BNB: avgCost = (10000 + 10)/1 = 10010
		// Sell with USDT: profit = (11000-10010)*1 using quoteQty (11000-11)=10989
		// Profit = (11000 - 10010) * 1 = 990
		assert.Equal(t, fixedpoint.NewFromInt(990).String(), stats.AccumulatedPnL.String())
	})

	t.Run("position reversal - long to short", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		now := time.Now()
		trades := []types.Trade{
			{
				ID:            1,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(10000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(10000),
				Fee:           fixedpoint.Zero,
				FeeCurrency:   "USDT",
				Time:          types.Time(now),
			},
			{
				ID:            2,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeSell,
				Price:         fixedpoint.NewFromInt(11000),
				Quantity:      fixedpoint.NewFromInt(2), // Sell more than we have
				QuoteQuantity: fixedpoint.NewFromInt(22000),
				Fee:           fixedpoint.Zero,
				FeeCurrency:   "USDT",
				Time:          types.Time(now.Add(time.Hour)),
			},
		}

		err := fixer.fixFromTrades(trades, nil, stats, position)

		assert.NoError(t, err)
		// After reversal, position should be short 1 BTC
		assert.Equal(t, fixedpoint.NewFromInt(-1).String(), position.Base.String())
		// Profit from closing long: (11000 - 10000) * 1 = 1000
		assert.Equal(t, fixedpoint.NewFromInt(1000).String(), stats.AccumulatedPnL.String())
	})

	t.Run("position reversal - short to long", func(t *testing.T) {
		position := types.NewPositionFromMarket(market)
		stats := types.NewProfitStats(market)
		fixer := newMockFixer()

		now := time.Now()
		trades := []types.Trade{
			{
				ID:            1,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeSell,
				Price:         fixedpoint.NewFromInt(10000),
				Quantity:      fixedpoint.NewFromInt(1),
				QuoteQuantity: fixedpoint.NewFromInt(10000),
				Fee:           fixedpoint.Zero,
				FeeCurrency:   "USDT",
				Time:          types.Time(now),
			},
			{
				ID:            2,
				Exchange:      types.ExchangeBinance,
				Symbol:        symbol,
				Side:          types.SideTypeBuy,
				Price:         fixedpoint.NewFromInt(9000),
				Quantity:      fixedpoint.NewFromInt(2), // Buy more than we need
				QuoteQuantity: fixedpoint.NewFromInt(18000),
				Fee:           fixedpoint.Zero,
				FeeCurrency:   "USDT",
				Time:          types.Time(now.Add(time.Hour)),
			},
		}

		err := fixer.fixFromTrades(trades, nil, stats, position)

		assert.NoError(t, err)
		// After reversal, position should be long 1 BTC
		assert.Equal(t, fixedpoint.NewFromInt(1).String(), position.Base.String())
		// Profit from closing short: (10000 - 9000) * 1 = 1000
		assert.Equal(t, fixedpoint.NewFromInt(1000).String(), stats.AccumulatedPnL.String())
	})
}
