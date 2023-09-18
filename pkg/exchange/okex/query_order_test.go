package okex

import (
	"context"
	"os"
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_QueryOrder(t *testing.T) {
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

	e := New(key, secret, passphrase)

	queryOrder := types.OrderQuery{
		Symbol:  "BTCUSDT",
		OrderID: "609869603774656544",
	}
	orderDetail, err := e.QueryOrder(context.Background(), queryOrder)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, orderDetail)
	}
	t.Logf("order detail: %+v", orderDetail)
}
