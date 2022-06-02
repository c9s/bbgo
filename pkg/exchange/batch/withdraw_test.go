package batch

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/testutil"
)

func TestWithdrawBatchQuery(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BINANCE")
	if !ok {
		t.Skip("binance api is not set")
	}

	ex := binance.New(key, secret)
	q := WithdrawBatchQuery{
		ExchangeTransferService: ex,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	now := time.Now()
	startTime := now.AddDate(0, -6, 0)
	endTime := now
	dataC, errC := q.Query(ctx, "", startTime, endTime)

	for withdraw := range dataC {
		t.Logf("%+v", withdraw)
	}

	err := <-errC
	assert.NoError(t, err)
}
