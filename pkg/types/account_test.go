package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccountLockAndUnlock(t *testing.T) {
	a := NewAccount()
	err := a.AddBalance("USDT", 1000.0)
	assert.NoError(t, err)

	balance, ok := a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, 1000.0)
	assert.Equal(t, balance.Locked, 0.0)

	err = a.LockBalance("USDT", 100.0)
	assert.NoError(t, err)

	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, 900.0)
	assert.Equal(t, balance.Locked, 100.0)

	err = a.UnlockBalance("USDT", 100.0)
	assert.NoError(t, err)
	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, 1000.0)
	assert.Equal(t, balance.Locked, 0.0)
}

func TestAccountLockAndUse(t *testing.T) {
	a := NewAccount()
	err := a.AddBalance("USDT", 1000.0)
	assert.NoError(t, err)

	balance, ok := a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, 1000.0)
	assert.Equal(t, balance.Locked, 0.0)

	err = a.LockBalance("USDT", 100.0)
	assert.NoError(t, err)

	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, 900.0)
	assert.Equal(t, balance.Locked, 100.0)

	err = a.UseLockedBalance("USDT", 100.0)
	assert.NoError(t, err)


	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, 900.0)
	assert.Equal(t, balance.Locked, 0.0)
}
