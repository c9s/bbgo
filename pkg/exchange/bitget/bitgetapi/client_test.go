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

	t.Run("GetSymbolsRequest", func(t *testing.T) {
		req := client.NewGetSymbolsRequest()
		symbols, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("symbols: %+v", symbols)
	})

	t.Run("GetTickerRequest", func(t *testing.T) {
		req := client.NewGetTickerRequest()
		req.Symbol("BTCUSDT_SPBL")
		ticker, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("ticker: %+v", ticker)
	})

	t.Run("GetDepthRequest", func(t *testing.T) {
		req := client.NewGetDepthRequest()
		req.Symbol("BTCUSDT_SPBL")
		depth, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("depth: %+v", depth)
	})

	t.Run("GetServerTimeRequest", func(t *testing.T) {
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

	t.Run("GetAccountTransfersRequest", func(t *testing.T) {
		req := client.NewGetAccountTransfersRequest()
		req.CoinId(1)
		req.FromType(AccountExchange)
		transfers, err := req.Do(ctx)

		assert.NoError(t, err)
		t.Logf("transfers: %+v", transfers)
	})
}
