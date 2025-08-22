package bitfinex

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
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

	markets, err := ex.QueryMarkets(ctx)
	if !assert.NoError(t, err) {
		return
	}

	market, ok := markets["BTCUSDT"]
	if !assert.True(t, ok, "BTCUSDT market not found") {
		return
	}

	t.Logf("market: %+v", market)

	canSell := false
	maxQtySell := Number(0.0)
	bals, err := ex.QueryAccountBalances(ctx)
	if assert.NoError(t, err) {
		assert.NotNil(t, bals)
		t.Logf("balances: %+v", bals)

		if baseBal, ok := bals["BTC"]; ok {
			if baseBal.Available.Compare(market.MinQuantity) > 0 {
				canSell = true
				maxQtySell = baseBal.Available
			}
		}
	}

	ticker, err := ex.QueryTicker(ctx, "BTCUSDT")
	if !assert.NoError(t, err) {
		return
	}

	err = ticker.Validate()
	if !assert.NoError(t, err) {
		return
	}

	var activeOrders []types.Order
	var takerOrder *types.Order

	if canSell {
		qty := fixedpoint.Min(Number(8.0).Div(ticker.Buy), maxQtySell)

		t.Logf("submitting sell order with max quantity: %s", qty)

		takerOrder, err = ex.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimit,
			Price:    ticker.Buy,
			Market:   market,
			Quantity: fixedpoint.Min(Number(8.0).Div(ticker.Buy), maxQtySell),
		})

		if assert.NoError(t, err) {
			assert.Equal(t, "BTCUSDT", takerOrder.Symbol)
			assert.True(t, !takerOrder.Price.IsZero())
			assert.True(t, !takerOrder.Quantity.IsZero())
			assert.Equal(t, types.SideTypeSell, takerOrder.Side)
			assert.Equal(t, types.OrderTypeLimit, takerOrder.Type)
			assert.Equal(t, types.OrderStatusNew, takerOrder.Status)
			activeOrders = append(activeOrders, *takerOrder)
		}
	} else {
		t.Logf("submitting buy order at %v", ticker.Sell)

		takerOrder, err = ex.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    ticker.Sell,
			Market:   market,
			Quantity: Number(10.0).Div(ticker.Sell),
		})

		if assert.NoError(t, err) {
			assert.Equal(t, "BTCUSDT", takerOrder.Symbol)
			assert.True(t, !takerOrder.Price.IsZero())
			assert.True(t, !takerOrder.Quantity.IsZero())
			assert.Equal(t, types.SideTypeBuy, takerOrder.Side)
			assert.Equal(t, types.OrderTypeLimit, takerOrder.Type)
			assert.Equal(t, types.OrderStatusNew, takerOrder.Status)
			activeOrders = append(activeOrders, *takerOrder)
		}
	}

	t.Cleanup(func() {
		t.Logf("cleanup: cancel %d active orders", len(activeOrders))
		for _, order := range activeOrders {
			t.Logf("cancel order: %+v", order)
			if err := ex.CancelOrders(ctx, order); err != nil {
				t.Logf("failed to cancel order: %v", err)
			} else {
				t.Logf("order canceled: %d", order.OrderID)
			}
		}
	})

	if isRecording {
		time.Sleep(1 * time.Second)
	}

	updatedOrder, err := ex.QueryOrder(ctx, takerOrder.AsQuery())
	if assert.NoError(t, err) {
		assert.Equal(t, takerOrder.Symbol, updatedOrder.Symbol)
		assert.Equal(t, takerOrder.OrderID, updatedOrder.OrderID)
		if canSell {
			assert.Equal(t, types.SideTypeSell, updatedOrder.Side)
		} else {
			assert.Equal(t, types.SideTypeBuy, updatedOrder.Side)
		}
		assert.False(t, updatedOrder.Price.IsZero())
		assert.False(t, updatedOrder.Quantity.IsZero())
		assert.True(t, updatedOrder.Quantity.Sign() > 0)
		t.Logf("updated order: %+v", updatedOrder)
	}

	trades, err := ex.QueryOrderTrades(ctx, takerOrder.AsQuery())
	if assert.NoError(t, err) {
		assert.NotEmpty(t, trades)
		for _, trade := range trades {
			assert.Equal(t, takerOrder.Symbol, trade.Symbol)
			assert.Equal(t, takerOrder.OrderID, trade.OrderID)
			if canSell {
				assert.Equal(t, types.SideTypeSell, trade.Side)
			} else {
				assert.Equal(t, types.SideTypeBuy, trade.Side)
			}

			assert.False(t, trade.Price.IsZero())
			assert.False(t, trade.Quantity.IsZero())
			assert.True(t, trade.Quantity.Sign() > 0)
		}
	}

	/*
		buyPrice := ticker.Buy.Mul(Number(0.9))
		makerOrder, err := ex.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Price:    buyPrice,
			Quantity: Number(100.0).Div(buyPrice),
		})
		if assert.NoError(t, err) {
			assert.Equal(t, "BTCUSDT", makerOrder.Symbol)
			assert.True(t, !makerOrder.Price.IsZero())
			assert.True(t, !makerOrder.Quantity.IsZero())
			assert.Equal(t, types.SideTypeBuy, makerOrder.Side)
			assert.Equal(t, types.OrderTypeLimit, makerOrder.Type)
			assert.Equal(t, types.OrderStatusNew, makerOrder.Status)
			activeOrders = append(activeOrders, *makerOrder)
		}
	*/
}
