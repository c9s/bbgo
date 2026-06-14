package binance

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_newClientOrderID(t *testing.T) {
	cID := newSpotClientOrderID("")
	assert.Len(t, cID, 32)
	strings.HasPrefix(cID, "x-"+spotBrokerID)

	cID = newSpotClientOrderID("myid1")
	assert.Equal(t, "myid1", cID)
}

func Test_new(t *testing.T) {
	ex := New("", "")
	assert.NotEmpty(t, ex)
	ctx := context.Background()
	ticker, err := ex.QueryTicker(ctx, "btcusdt")
	if len(os.Getenv("GITHUB_CI")) > 0 {
		// Github action runs in the US, and therefore binance api is not accessible
		assert.Empty(t, ticker)
		assert.Error(t, err)
	} else {
		assert.NotEmpty(t, ticker)
		assert.NoError(t, err)
	}
}

func Test_QueryPositionRisk(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BINANCE")
	if !ok {
		t.SkipNow()
		return
	}

	ex := New(key, secret)
	ex.UseFutures()

	assert.NotEmpty(t, ex)
	ctx := context.Background()

	// Test case 1: Query all positions
	positions, err := ex.QueryPositionRisk(ctx)
	assert.NoError(t, err)

	// Test case 2: Query specific symbol position
	positions, err = ex.QueryPositionRisk(ctx, "BTCUSDT")
	assert.NoError(t, err)
	if len(positions) > 0 {
		assert.Equal(t, "BTCUSDT", positions[0].Symbol)
	}

	// Test case 3: Query multiple symbols
	positions, err = ex.QueryPositionRisk(ctx, "BTCUSDT", "ETHUSDT")
	assert.NoError(t, err)
	if len(positions) > 0 {
		symbols := make(map[string]bool)
		for _, pos := range positions {
			symbols[pos.Symbol] = true
		}
		assert.True(t, symbols["BTCUSDT"] || symbols["ETHUSDT"])
	}

}

func Test_QueryFuturesMarkPriceKLines(t *testing.T) {
	ex := New("", "")
	ex.UseFutures()
	assert.NotEmpty(t, ex)
	ctx := context.Background()

	klines, err := ex.futuresClient2.NewFuturesMarkPriceKlinesRequest().
		Symbol("BTCUSDT").
		Interval(types.Interval1h).
		Limit(10).
		Do(ctx)

	if len(os.Getenv("GITHUB_CI")) > 0 {
		// Github action runs in the US, and therefore binance api is not accessible
		assert.Error(t, err)
	} else {
		assert.NoError(t, err)
		assert.NotEmpty(t, klines)
		if len(klines) > 0 {
			k := klines[0]
			assert.False(t, k.OpenTime.Time().IsZero())
			assert.False(t, k.CloseTime.Time().IsZero())
			assert.False(t, k.Close.IsZero())
		}
	}
}

func Test_QueryFuturesIndexPriceKLines(t *testing.T) {
	ex := New("", "")
	ex.UseFutures()
	assert.NotEmpty(t, ex)
	ctx := context.Background()

	klines, err := ex.futuresClient2.NewFuturesIndexPriceKlinesRequest().
		Pair("BTCUSDT").
		Interval(types.Interval1h).
		Limit(10).
		Do(ctx)

	if len(os.Getenv("GITHUB_CI")) > 0 {
		// Github action runs in the US, and therefore binance api is not accessible
		assert.Error(t, err)
	} else {
		assert.NoError(t, err)
		assert.NotEmpty(t, klines)
		if len(klines) > 0 {
			k := klines[0]
			assert.False(t, k.OpenTime.Time().IsZero())
			assert.False(t, k.CloseTime.Time().IsZero())
			assert.False(t, k.Close.IsZero())
		}
	}
}
