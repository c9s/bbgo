package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func Test_newMarketMemCache(t *testing.T) {
	cache := newMarketMemCache()
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.markets)
}

func Test_marketMemCache_GetSet(t *testing.T) {
	cache := newMarketMemCache()
	cache.Set("max", types.MarketMap{
		"btctwd": types.Market{
			Symbol:      "btctwd",
			LocalSymbol: "btctwd",
		},
		"ethtwd": types.Market{
			Symbol:      "ethtwd",
			LocalSymbol: "ethtwd",
		},
	})
	markets, ok := cache.Get("max")
	assert.True(t, ok)

	btctwd, ok := markets["btctwd"]
	assert.True(t, ok)
	ethtwd, ok := markets["ethtwd"]
	assert.True(t, ok)
	assert.Equal(t, types.Market{
		Symbol:      "btctwd",
		LocalSymbol: "btctwd",
	}, btctwd)
	assert.Equal(t, types.Market{
		Symbol:      "ethtwd",
		LocalSymbol: "ethtwd",
	}, ethtwd)

	_, ok = cache.Get("binance")
	assert.False(t, ok)

	expired := cache.IsOutdated("max")
	assert.False(t, expired)

	detailed := cache.markets["max"]
	detailed.updatedAt = time.Now().Add(-2 * memCacheExpiry)
	cache.markets["max"] = detailed
	expired = cache.IsOutdated("max")
	assert.True(t, expired)

	expired = cache.IsOutdated("binance")
	assert.True(t, expired)
}

func Test_loadMarketsFromMem(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockEx := mocks.NewMockExchange(mockCtrl)
	mockEx.EXPECT().Name().Return(types.ExchangeName("max")).AnyTimes()
	mockEx.EXPECT().QueryMarkets(gomock.Any()).Return(nil, errors.New("faked")).Times(1)
	mockEx.EXPECT().QueryMarkets(gomock.Any()).Return(types.MarketMap{
		"btctwd": types.Market{
			Symbol:      "btctwd",
			LocalSymbol: "btctwd",
		},
		"ethtwd": types.Market{
			Symbol:      "ethtwd",
			LocalSymbol: "ethtwd",
		},
	}, nil).Times(1)

	for i := 0; i < 10; i++ {
		markets, err := loadMarketsFromMem(context.Background(), mockEx)
		assert.NoError(t, err)

		btctwd, ok := markets["btctwd"]
		assert.True(t, ok)
		ethtwd, ok := markets["ethtwd"]
		assert.True(t, ok)
		assert.Equal(t, types.Market{
			Symbol:      "btctwd",
			LocalSymbol: "btctwd",
		}, btctwd)
		assert.Equal(t, types.Market{
			Symbol:      "ethtwd",
			LocalSymbol: "ethtwd",
		}, ethtwd)
	}

	globalMarketMemCache = newMarketMemCache() // reset the global cache
}
