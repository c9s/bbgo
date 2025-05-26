package kucoin

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestExchangeOrderQueryService(t *testing.T) {
	exchange := getExchangeOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// create order
	createOrder, err := exchange.SubmitOrder(ctx, types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Price:    fixedpoint.NewFromFloat(100),
		Quantity: fixedpoint.NewFromFloat(1.0),
	})
	assert.NoError(t, err)
	// query order
	order, err := exchange.QueryOrder(ctx, types.OrderQuery{
		OrderUUID: createOrder.UUID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, order)
	// cancel order
	err = exchange.CancelOrders(ctx, *createOrder)
	assert.NoError(t, err)

	// no trades
	trades, err := exchange.QueryOrderTrades(ctx, types.OrderQuery{
		OrderUUID: createOrder.UUID,
	})
	assert.NoError(t, err)
	assert.Empty(t, trades)

}

func getExchangeOrSkip(t *testing.T) *Exchange {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "KUCOIN")
	if !ok {
		t.SkipNow()
		return nil
	}

	return New(key, secret, passphrase)
}
