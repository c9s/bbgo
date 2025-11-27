package maxapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func TestWithdrawal(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	client := NewRestClientDefault()
	client.Auth(key, secret)

	t.Run("v2/withdrawals", func(t *testing.T) {
		req := client.NewGetWithdrawalHistoryRequest()
		req.Currency("usdt")
		withdrawals, err := req.Do(ctx)
		if assert.NoError(t, err) {
			assert.NotEmpty(t, withdrawals)
		}
	})
}
