package bitgetapi

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func getTestClientOrSkip(t *testing.T) *RestClient {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITGET")
	if !ok {
		t.Skip("BITGET_* env vars are not configured")
		return nil
	}

	client := NewClient()
	client.Auth(key, secret, os.Getenv("BITGET_API_PASSPHRASE"))
	return client
}

func TestClient(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	t.Run("GetAllTickersRequest", func(t *testing.T) {
		req := client.NewGetAllTickersRequest()
		tickers, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("tickers: %+v", tickers)
	})

	t.Run("GetTickerRequest", func(t *testing.T) {
		req := client.NewGetTickerRequest()
		req.Symbol("BTCUSDT_SPBL")
		ticker, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("ticker: %+v", ticker)
	})

	t.Run("GetServerTime", func(t *testing.T) {
		req := client.NewGetServerTimeRequest()
		serverTime, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("time: %+v", serverTime)
	})

	t.Run("GetAccountAssetsRequest", func(t *testing.T) {
		req := client.NewGetAccountAssetsRequest()
		assets, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("assets: %+v", assets)
	})
}
