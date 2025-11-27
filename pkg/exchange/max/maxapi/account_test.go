package maxapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func TestAccountService_GetWithdrawHistoryRequest(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClientDefault()
	client.Auth(key, secret)

	req := client.NewGetWithdrawalHistoryRequest()
	req.Currency("usdt")
	withdraws, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, withdraws)
	assert.NotEmpty(t, withdraws)
	t.Logf("withdraws: %+v", withdraws)
}

func TestAccountService_NewGetDepositHistoryRequest(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClientDefault()
	client.Auth(key, secret)

	req := client.NewGetDepositHistoryRequest()
	req.Currency("usdt")
	deposits, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, deposits)
	assert.NotEmpty(t, deposits)
	t.Logf("deposits: %+v", deposits)
}
