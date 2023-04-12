package max

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithdrawal(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	client := NewRestClient(ProductionAPIURL)
	client.Auth(key, secret)

	t.Run("v2/withdrawals", func(t *testing.T) {
		req := client.NewGetWithdrawalHistoryRequest()
		req.Currency("usdt")
		withdrawals, err := req.Do(ctx)
		assert.NoError(t, err)
		assert.NotEmpty(t, withdrawals)
	})
}
