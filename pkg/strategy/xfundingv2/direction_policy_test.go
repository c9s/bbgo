package xfundingv2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestNewDirectionPolicy(t *testing.T) {
	market := Market("BTCUSDT")

	t.Run("rejects unsupported direction", func(t *testing.T) {
		_, err := newDirectionPolicy(types.PositionClosed, market)
		assert.Error(t, err)
	})

	t.Run("accepts short", func(t *testing.T) {
		p, err := newDirectionPolicy(types.PositionShort, market)
		assert.NoError(t, err)
		assert.Equal(t, types.PositionShort, p.Direction)
	})

	t.Run("accepts long", func(t *testing.T) {
		p, err := newDirectionPolicy(types.PositionLong, market)
		assert.NoError(t, err)
		assert.Equal(t, types.PositionLong, p.Direction)
	})
}

func TestDirectionPolicy_CollateralAsset(t *testing.T) {
	market := Market("BTCUSDT")

	short, _ := newDirectionPolicy(types.PositionShort, market)
	long, _ := newDirectionPolicy(types.PositionLong, market)

	assert.Equal(t, market.BaseCurrency, short.CollateralAsset())
	assert.Equal(t, market.QuoteCurrency, long.CollateralAsset())
}

func TestDirectionPolicy_TransferAmountFromSpotTrade(t *testing.T) {
	market := Market("BTCUSDT")

	t.Run("short returns trade.Quantity (base)", func(t *testing.T) {
		p, _ := newDirectionPolicy(types.PositionShort, market)
		trade := types.Trade{
			Quantity:      Number(0.5),
			QuoteQuantity: Number(25000),
			Fee:           Number(25),
			FeeCurrency:   "USDT",
		}
		assert.Equal(t, Number(0.5), p.TransferAmountFromSpotTrade(trade))
	})

	t.Run("long deducts quote fee", func(t *testing.T) {
		p, _ := newDirectionPolicy(types.PositionLong, market)
		trade := types.Trade{
			Quantity:      Number(0.5),
			QuoteQuantity: Number(25000),
			Fee:           Number(25),
			FeeCurrency:   "USDT",
		}
		assert.Equal(t, Number(24975), p.TransferAmountFromSpotTrade(trade))
	})

	t.Run("long with non-quote fee returns full quote", func(t *testing.T) {
		p, _ := newDirectionPolicy(types.PositionLong, market)
		trade := types.Trade{
			Quantity:      Number(0.5),
			QuoteQuantity: Number(25000),
			Fee:           Number(0.01),
			FeeCurrency:   "BNB",
		}
		assert.Equal(t, Number(25000), p.TransferAmountFromSpotTrade(trade))
	})

	t.Run("long clamps negative result to zero", func(t *testing.T) {
		p, _ := newDirectionPolicy(types.PositionLong, market)
		trade := types.Trade{
			QuoteQuantity: Number(1),
			Fee:           Number(5),
			FeeCurrency:   "USDT",
		}
		assert.Equal(t, fixedpoint.Zero, p.TransferAmountFromSpotTrade(trade))
	})
}

func TestDirectionPolicy_TransferAmountFromFuturesTrade(t *testing.T) {
	market := Market("BTCUSDT")

	t.Run("short returns base quantity", func(t *testing.T) {
		p, _ := newDirectionPolicy(types.PositionShort, market)
		trade := types.Trade{Quantity: Number(0.5), QuoteQuantity: Number(25000)}
		assert.Equal(t, Number(0.5), p.TransferAmountFromFuturesTrade(trade))
	})

	t.Run("long returns quote quantity", func(t *testing.T) {
		p, _ := newDirectionPolicy(types.PositionLong, market)
		trade := types.Trade{Quantity: Number(0.5), QuoteQuantity: Number(25000)}
		assert.Equal(t, Number(25000), p.TransferAmountFromFuturesTrade(trade))
	})
}

func TestDirectionPolicy_Signs(t *testing.T) {
	market := Market("BTCUSDT")
	short, _ := newDirectionPolicy(types.PositionShort, market)
	long, _ := newDirectionPolicy(types.PositionLong, market)

	assert.Equal(t, 1, short.SpotSign())
	assert.Equal(t, -1, short.FuturesSign())
	assert.Equal(t, -1, long.SpotSign())
	assert.Equal(t, 1, long.FuturesSign())
}
