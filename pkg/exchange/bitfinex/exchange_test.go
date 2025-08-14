package bitfinex

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testing/httptesting"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

func TestExchange_submitOrderAndCancel(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ex *Exchange
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITFINEX")
	ex = New(key, secret)

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, ex.client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("BITFINEX api key is not configured, skipping integration test")
	}

	t.Run("SubmitOrderAndCancel", func(t *testing.T) {
		order, err := ex.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    Number(10_000.0),
			Quantity: Number(0.0001),
		})

		assert.Equal(t, "BTCUSDT", order.Symbol)
		assert.True(t, !order.Price.IsZero())
		assert.True(t, !order.Quantity.IsZero())
		assert.Equal(t, types.OrderStatusNew, order.Status)

		if assert.NoError(t, err) {
			err2 := ex.CancelOrders(ctx, *order)
			assert.NoError(t, err2)
		}
	})
}
