package okex

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_QueryClosedOrders(t *testing.T) {
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
		Symbol: "BTC-USDT",
	}

	// test by order id as a cursor
	closedOrder, err := e.QueryClosedOrders(context.Background(), string(queryOrder.Symbol), time.Time{}, time.Time{}, 0)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, closedOrder)
	}
	// test by time interval
	closedOrder, err = e.QueryClosedOrders(context.Background(), string(queryOrder.Symbol), time.Now().Add(-30*24*time.Hour), time.Now(), 0)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, closedOrder)
	}
	t.Logf("closed order detail: %+v", closedOrder)
}
