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

func TestClient_GetAccountAssetsRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	req := client.NewGetAccountAssetsRequest()
	assets, err := req.Do(ctx)
	assert.NoError(t, err)
	t.Logf("assets: %+v", assets)
}

func TestClient_GetTickerRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	req := client.NewGetTickerRequest()
	req.Symbol("BTCUSDT_SPBL")
	ticker, err := req.Do(ctx)
	assert.NoError(t, err)
	t.Logf("ticker: %+v", ticker)
}
