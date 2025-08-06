package bfxapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func TestClient(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITFINEX")
	if !ok {
		t.Skipf("BITFINEX api key is not configured, skipping integration test")
	}

	client := NewClient()
	client.Auth(key, secret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("GetTickerRequest", func(t *testing.T) {
		req := client.NewGetTickerRequest()
		req.Symbol("tBTCUSD")
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("ticker response: %+v", resp)
	})

	t.Run("GetWalletsRequest", func(t *testing.T) {
		req := client.NewGetWalletsRequest()
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("wallets response: %+v", resp)
		assert.NotEmpty(t, resp, "expected non-empty wallets response")
	})
}
