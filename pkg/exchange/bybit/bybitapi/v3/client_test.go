package v3

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/testutil"
)

func getTestClientOrSkip(t *testing.T) *bybitapi.RestClient {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BYBIT")
	if !ok {
		t.Skip("BYBIT_* env vars are not configured")
		return nil
	}

	client, err := bybitapi.NewClient()
	assert.NoError(t, err)
	client.Auth(key, secret)
	return client
}

func TestClient(t *testing.T) {
	client := getTestClientOrSkip(t)
	v3Client := Client{Client: client}
	ctx := context.Background()

	t.Run("GetTradeRequest", func(t *testing.T) {
		startTime := time.Date(2023, 7, 27, 16, 13, 9, 0, time.UTC)
		apiResp, err := v3Client.NewGetTradesRequest().Symbol("BTCUSDT").StartTime(startTime).Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %+v", apiResp)
	})
}
