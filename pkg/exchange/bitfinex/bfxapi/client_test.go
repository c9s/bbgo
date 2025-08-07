package bfxapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func TestGetTickerRequest(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITFINEX")
	if !ok {
		t.Skipf("BITFINEX api key is not configured, skipping integration test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient()
	client.Credentials(key, secret)
	req := client.NewGetTickerRequest()
	req.Symbol("tBTCUSD")
	resp, err := req.Do(ctx)
	assert.NoError(t, err)
	t.Logf("ticker response: %+v", resp)
}
