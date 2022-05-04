package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestBalanceMap_Add(t *testing.T) {
	var bm = BalanceMap{}
	var bm2 = bm.Add(BalanceMap{
		"BTC": Balance{
			Currency:  "BTC",
			Available: fixedpoint.MustNewFromString("10.0"),
			Locked:    fixedpoint.MustNewFromString("0"),
			Borrowed:  0,
			Interest:  0,
			NetAsset:  fixedpoint.MustNewFromString("10.0"),
		},
	})
	assert.Len(t, bm2, 1)
}

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
