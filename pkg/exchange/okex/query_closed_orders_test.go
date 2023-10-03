package okex

import (
	"context"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_QueryClosedOrders(t *testing.T) {

	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, types.ExchangeOKEx.String())
	if !ok {
		t.Skip("Please configure all credentials about OKEX")
	}

	e := New(key, secret, passphrase)

	queryOrder := types.OrderQuery{
		Symbol: "BTCUSDT",
	}

	// test by order id as a cursor
	/*
		closedOrder, err := e.QueryClosedOrders(context.Background(), string(queryOrder.Symbol), time.Time{}, time.Time{}, 609869603774656544)
		if assert.NoError(t, err) {
			assert.NotEmpty(t, closedOrder)
		}
		t.Logf("closed order detail: %+v", closedOrder)
	*/
	// test by time interval
	closedOrder, err := e.QueryClosedOrders(context.Background(), string(queryOrder.Symbol), time.Now().Add(-90*24*time.Hour), time.Now(), 0)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, closedOrder)
	}
	t.Logf("closed order detail: %+v", closedOrder)
	// test by no parameter
	closedOrder, err = e.QueryClosedOrders(context.Background(), string(queryOrder.Symbol), time.Time{}, time.Time{}, 0)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, closedOrder)
	}
	t.Logf("closed order detail: %+v", closedOrder)
	// test by time interval (boundary test)
	closedOrder, err = e.QueryClosedOrders(context.Background(), string(queryOrder.Symbol), time.Unix(1694155903, 999), time.Now(), 0)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, closedOrder)
	}
	t.Logf("closed order detail: %+v", closedOrder)
	// test by time interval (boundary test)
	closedOrder, err = e.QueryClosedOrders(context.Background(), string(queryOrder.Symbol), time.Unix(1694154903, 999), time.Unix(1694155904, 0), 0)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, closedOrder)
	}
	t.Logf("closed order detail: %+v", closedOrder)
	// test by time interval and order id together
	/*
		closedOrder, err = e.QueryClosedOrders(context.Background(), string(queryOrder.Symbol), time.Unix(1694154903, 999), time.Now(), 609869603774656544)
		if assert.NoError(t, err) {
			assert.NotEmpty(t, closedOrder)
		}
		t.Logf("closed order detail: %+v", closedOrder)
	*/
}
