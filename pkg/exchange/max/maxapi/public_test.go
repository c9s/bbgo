package max

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublicService(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	client := NewRestClient(ProductionAPIURL)
	_ = key
	_ = secret

	t.Run("v2/timestamp", func(t *testing.T) {
		req := client.NewGetTimestampRequest()
		serverTimestamp, err := req.Do(ctx)
		assert.NoError(t, err)
		assert.NotZero(t, serverTimestamp)
	})

	t.Run("v2/tickers", func(t *testing.T) {
		req := client.NewGetTickersRequest()
		tickers, err := req.Do(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, tickers)
		assert.NotEmpty(t, tickers)
		assert.NotEmpty(t, tickers["btcusdt"])
	})

	t.Run("v2/ticker/:market", func(t *testing.T) {
		req := client.NewGetTickerRequest()
		req.Market("btcusdt")
		ticker, err := req.Do(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, ticker)
		assert.NotEmpty(t, ticker.Sell)
	})
}
