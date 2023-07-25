package bybitapi

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

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BYBIT")
	if !ok {
		t.Skip("BYBIT_* env vars are not configured")
		return nil
	}

	client, err := NewClient()
	assert.NoError(t, err)
	client.Auth(key, secret)
	return client
}

func TestClient(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	t.Run("GetAccountInfoRequest", func(t *testing.T) {
		req := client.NewGetAccountRequest()
		accountInfo, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("accountInfo: %+v", accountInfo)
	})

	t.Run("GetInstrumentsInfoRequest", func(t *testing.T) {
		req := client.NewGetInstrumentsInfoRequest()
		instrumentsInfo, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("instrumentsInfo: %+v", instrumentsInfo)
	})

	t.Run("GetTicker", func(t *testing.T) {
		req := client.NewGetTickersRequest()
		apiResp, err := req.Symbol("BTCUSDT").Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %+v", apiResp)

		req = client.NewGetTickersRequest()
		tickers, err := req.Symbol("BTCUSDT").DoWithResponseTime(ctx)
		assert.NoError(t, err)
		t.Logf("tickers: %+v", tickers)
	})
}
