package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestAccountLockAndUnlock(t *testing.T) {
	a := NewAccount()
	err := a.AddBalance("USDT", 1000)
	assert.NoError(t, err)

	balance, ok := a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.Value(1000))
	assert.Equal(t, balance.Locked, fixedpoint.Value(0))

	err = a.LockBalance("USDT", fixedpoint.Value(100))
	assert.NoError(t, err)

	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.Value(900))
	assert.Equal(t, balance.Locked, fixedpoint.Value(100))

	err = a.UnlockBalance("USDT", 100)
	assert.NoError(t, err)
	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.Value(1000))
	assert.Equal(t, balance.Locked, fixedpoint.Value(0))
}

func TestAccountLockAndUse(t *testing.T) {
	a := NewAccount()
	err := a.AddBalance("USDT", 1000)
	assert.NoError(t, err)

	balance, ok := a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.Value(1000))
	assert.Equal(t, balance.Locked, fixedpoint.Value(0))

	err = a.LockBalance("USDT", 100)
	assert.NoError(t, err)

	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.Value(900))
	assert.Equal(t, balance.Locked, fixedpoint.Value(100))

	err = a.UseLockedBalance("USDT", 100)
	assert.NoError(t, err)

	balance, ok = a.Balance("USDT")
	assert.True(t, ok)
	assert.Equal(t, balance.Available, fixedpoint.Value(900))
	assert.Equal(t, balance.Locked, fixedpoint.Value(0))
}
