package okex

import (
	"context"
	"os"
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_QueryOrderTrades(t *testing.T) {
	key := os.Getenv("OKEX_API_KEY")
	secret := os.Getenv("OKEX_API_SECRET")
	passphrase := os.Getenv("OKEX_API_PASSPHRASE")
	if len(key) == 0 && len(secret) == 0 {
		t.Skip("api key/secret are not configured")
		return
	}
	if len(passphrase) == 0 {
		t.Skip("passphrase are not configured")
		return
	}

	e, err := New(key, secret, passphrase)
	assert.NoError(t, err)

	queryOrder := types.OrderQuery{
		OrderID: "609869603774656544",
	}
	transactionDetail, err := e.QueryOrderTrades(context.Background(), queryOrder)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, transactionDetail)
	}
	t.Logf("transaction detail: %+v", transactionDetail)
}
