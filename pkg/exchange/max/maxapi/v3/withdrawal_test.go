package v3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/testutil"
)

func TestWithdrawal(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	legecyClient := maxapi.NewRestClientDefault()
	legecyClient.Auth(key, secret)
	client := NewClient(legecyClient)

	t.Run("v3/withdrawals", func(t *testing.T) {
		req := client.NewGetWithdrawalHistoryRequest()
		req.Currency("usdt")
		withdrawals, err := req.Do(ctx)
		if assert.NoError(t, err) {
			assert.NotEmpty(t, withdrawals)
		}
	})
}
