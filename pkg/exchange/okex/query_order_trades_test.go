package okex

import (
	"context"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_QueryOrderTrades(t *testing.T) {

	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.Skip("Please configure all credentials about OKEX")
	}

	e := New(key, secret, passphrase)

	queryOrder := types.OrderQuery{
		OrderID: "609869603774656544",
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	transactionDetail, err := e.QueryOrderTrades(ctx, queryOrder)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
	t.Logf("transaction detail: %+v", transactionDetail)
	queryOrder = types.OrderQuery{
		Symbol: "BTC-USDT",
	}
	transactionDetail, err = e.QueryOrderTrades(ctx, queryOrder)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
	t.Logf("transaction detail: %+v", transactionDetail)
}
