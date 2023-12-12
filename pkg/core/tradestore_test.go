package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestTradeStore_isCoolTrade(t *testing.T) {
	now := time.Now()
	store := NewTradeStore()
	store.lastTradeTime = now.Add(-2 * time.Hour)
	ok := store.isCoolTrade(types.Trade{
		Time: types.Time(now),
	})
	assert.True(t, ok)

	store.lastTradeTime = now.Add(-2 * time.Minute)
	ok = store.isCoolTrade(types.Trade{
		Time: types.Time(now),
	})
	assert.False(t, ok)
}

func TestTradeStore_Prune(t *testing.T) {
	now := time.Now()
	store := NewTradeStore()
	store.Add(
		types.Trade{ID: 1, Time: types.Time(now.Add(-25 * time.Hour))},
		types.Trade{ID: 2, Time: types.Time(now.Add(-2 * time.Hour))},
		types.Trade{ID: 3, Time: types.Time(now.Add(-2 * time.Minute))},
		types.Trade{ID: 4, Time: types.Time(now.Add(-1 * time.Minute))},
	)
	store.Prune(now)
	assert.Equal(t, 3, len(store.trades))
}
