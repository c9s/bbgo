package max

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTradeService(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClient(ProductionAPIURL)
	client.Auth(key, secret)

	t.Run("default timestamp", func(t *testing.T) {
		req := client.TradeService.NewGetPrivateTradeRequest()
		until := time.Now().AddDate(0, -6, 0)

		trades, err := req.Market("btcusdt").
			Timestamp(until).
			Do(ctx)
		if assert.NoError(t, err) {
			assert.NotEmptyf(t, trades, "got %d trades", len(trades))
			for _, td := range trades {
				t.Logf("trade: %+v", td)
				assert.True(t, td.CreatedAtMilliSeconds.Time().Before(until))
			}
		}
	})

	t.Run("desc and pagination = false", func(t *testing.T) {
		req := client.TradeService.NewGetPrivateTradeRequest()
		trades, err := req.Market("btcusdt").
			Pagination(false).
			OrderBy("asc").
			Do(ctx)

		if assert.NoError(t, err) {
			assert.NotEmptyf(t, trades, "got %d trades", len(trades))
			for _, td := range trades {
				t.Logf("trade: %+v", td)
			}
		}
	})
}
