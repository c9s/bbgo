package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestAccountLockAndUnlock(t *testing.T) {
	a := NewAccount()
	a.AddBalance("USDT", fixedpoint.NewFromInt(1000))

	var err error
	balance, ok := a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.NewFromInt(1000))
	assert.Equal(t, balance.Locked, fixedpoint.Zero)

	err = a.LockBalance("USDT", fixedpoint.NewFromInt(100))
	assert.NoError(t, err)

	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.NewFromInt(900))
	assert.Equal(t, balance.Locked, fixedpoint.NewFromInt(100))

	err = a.UnlockBalance("USDT", fixedpoint.NewFromInt(100))
	assert.NoError(t, err)
	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.NewFromInt(1000))
	assert.Equal(t, balance.Locked, fixedpoint.Zero)
}

func TestAccountLockAndUse(t *testing.T) {
	a := NewAccount()
	a.AddBalance("USDT", fixedpoint.NewFromInt(1000))

	var err error
	balance, ok := a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.NewFromInt(1000))
	assert.Equal(t, balance.Locked, fixedpoint.Zero)

	err = a.LockBalance("USDT", fixedpoint.NewFromInt(100))
	assert.NoError(t, err)

	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.NewFromInt(900))
	assert.Equal(t, balance.Locked, fixedpoint.NewFromInt(100))

	err = a.UseLockedBalance("USDT", fixedpoint.NewFromInt(100))
	assert.NoError(t, err)

	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.NewFromInt(900))
	assert.Equal(t, balance.Locked, fixedpoint.Zero)
}

func TestUpdateFuturesPositions_MergeAndOverride(t *testing.T) {
	a := NewAccount()

	btcPosLong := NewPositionKey("BTCUSDT", PositionLong)
	ethPosShort := NewPositionKey("ETHUSDT", PositionShort)
	initial := FuturesPositionMap{
		btcPosLong: {
			Symbol:        "BTCUSDT",
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
			Base:          fixedpoint.NewFromInt(1),
		},
	}
	a.UpdateFuturesPositions(initial)

	update := FuturesPositionMap{
		btcPosLong: {
			Symbol:        "BTCUSDT",
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
			Base:          fixedpoint.NewFromInt(2), // override base
		},
		ethPosShort: {
			Symbol:        "ETHUSDT",
			BaseCurrency:  "ETH",
			QuoteCurrency: "USDT",
			Base:          fixedpoint.NewFromInt(5),
		},
	}
	a.UpdateFuturesPositions(update)

	assert.NotNil(t, a.FuturesInfo)
	assert.NotNil(t, a.FuturesInfo.Positions)

	btc, ok := a.FuturesInfo.Positions[btcPosLong]
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromInt(2), btc.Base)

	eth, ok := a.FuturesInfo.Positions[ethPosShort]
	assert.True(t, ok)
	assert.Equal(t, fixedpoint.NewFromInt(5), eth.Base)
}
