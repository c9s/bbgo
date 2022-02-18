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
