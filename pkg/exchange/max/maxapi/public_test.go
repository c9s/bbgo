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

	t.Run("v2/markets", func(t *testing.T) {
		req := client.NewGetMarketsRequest()
		markets, err := req.Do(context.Background())
		assert.NoError(t, err)
		if assert.NotEmpty(t, markets) {
			assert.NotZero(t, markets[0].MinBaseAmount)
			assert.NotZero(t, markets[0].MinQuoteAmount)
			assert.NotEmpty(t, markets[0].Name)
			assert.NotEmpty(t, markets[0].ID)
			assert.NotEmpty(t, markets[0].BaseUnit)
			assert.NotEmpty(t, markets[0].QuoteUnit)
			t.Logf("%+v", markets[0])
		}
	})

	t.Run("v2/k", func(t *testing.T) {
		req := client.NewGetKLinesRequest()
		data, err := req.Market("btcusdt").Period(int(60)).Limit(100).Do(ctx)
		assert.NoError(t, err)
		if assert.NotEmpty(t, data) {
			assert.NotEmpty(t, data[0])
			assert.Len(t, data[0], 6)
		}
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
