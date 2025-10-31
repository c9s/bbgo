package max

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

func TestExchange_recoverOrder(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	ex := New(key, secret, "")

	_, err := ex.recoverOrder(ctx, types.SubmitOrder{
		Symbol:        "BTCUSDT",
		ClientOrderID: "test" + strconv.FormatInt(time.Now().UnixMilli(), 10),
	}, nil)
	t.Logf("recover order error: %v", err)
	assert.Nil(t, err, "order should be nil if not found")

	orderForm := types.SubmitOrder{
		Symbol:        "BTCUSDT",
		ClientOrderID: "test" + strconv.FormatInt(time.Now().UnixMilli(), 10),
		Type:          types.OrderTypeLimit,
		Side:          types.SideTypeBuy,
		Price:         Number(90_000),
		Quantity:      Number(0.001),
	}

	order, err := ex.recoverOrder(ctx, orderForm, nil)

	if assert.NoError(t, err) {
		t.Logf("order: %+v", order)
		assert.Nil(t, order, "order should be nil if not found")

		order, err = ex.SubmitOrder(ctx, orderForm)
		if assert.NoError(t, err) {
			t.Logf("submitted order: %+v", order)
			if assert.NotNil(t, order) {
				err = ex.CancelOrders(ctx, *order)
				assert.NoError(t, err)

				order2, err2 := ex.recoverOrder(ctx, orderForm, nil)

				t.Logf("recovered order: %+v", order2)
				assert.NoError(t, err2)
				assert.NotNil(t, order2)
			}
		}
	}
}

func TestExchange_submitOrderAndCancel(t *testing.T) {
	// httptesting.AlwaysRecord = true
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ex := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, ex.client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	// patch client
	restClient := ex.v3client.RestClient
	restClient.HttpClient = ex.client.HttpClient

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
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
		qty := fixedpoint.Min(Number(20.0).Div(ticker.Buy), maxQtySell)
		t.Logf("submitting sell order with max quantity: %s", qty)
		takerOrder, err = ex.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimit,
			Price:    ticker.Buy,
			Market:   market,
			Quantity: qty,
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
}

func TestExchange_buildTimeRangeOnlyTradesRequest(t *testing.T) {
	ex := New("key", "secret", "")

	t.Run("spotWalletWithStartTime", func(t *testing.T) {
		startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		options := &types.TradeQueryOptions{
			StartTime: &startTime,
		}

		req := ex.buildTimeRangeOnlyTradesRequest("BTCUSDT", options)

		// Check query parameters
		params, err := req.GetParametersQuery()
		assert.NoError(t, err)
		assert.Equal(t, "btcusdt", params.Get("market"))
		assert.Equal(t, "asc", params.Get("order"))
		// Timestamp is in milliseconds
		assert.Equal(t, strconv.FormatInt(startTime.UnixMilli(), 10), params.Get("timestamp"))
	})

	t.Run("spotWalletWithEndTime", func(t *testing.T) {
		endTime := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
		options := &types.TradeQueryOptions{
			EndTime: &endTime,
		}

		req := ex.buildTimeRangeOnlyTradesRequest("BTCUSDT", options)

		// Check query parameters
		params, err := req.GetParametersQuery()
		assert.NoError(t, err)
		assert.Equal(t, "btcusdt", params.Get("market"))
		assert.Equal(t, "desc", params.Get("order"))
		// Timestamp is in milliseconds
		assert.Equal(t, strconv.FormatInt(endTime.UnixMilli(), 10), params.Get("timestamp"))
	})

	t.Run("marginWalletWithStartTime", func(t *testing.T) {
		ex.MarginSettings.IsMargin = true
		defer func() { ex.MarginSettings.IsMargin = false }()

		startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		options := &types.TradeQueryOptions{
			StartTime: &startTime,
		}

		req := ex.buildTimeRangeOnlyTradesRequest("BTCUSDT", options)

		// Check slug parameters (wallet type)
		slugParams, err := req.GetSlugParameters()
		assert.NoError(t, err)
		// WalletType is a custom type, so compare as string
		assert.Equal(t, "m", string(slugParams["walletType"].(maxapi.WalletType)))

		// Check query parameters
		params, err := req.GetParametersQuery()
		assert.NoError(t, err)
		assert.Equal(t, "btcusdt", params.Get("market"))
		assert.Equal(t, "asc", params.Get("order"))
	})

	t.Run("noTimeRangeSpecified", func(t *testing.T) {
		options := &types.TradeQueryOptions{}

		req := ex.buildTimeRangeOnlyTradesRequest("BTCUSDT", options)

		// Check query parameters
		params, err := req.GetParametersQuery()
		assert.NoError(t, err)
		assert.Equal(t, "btcusdt", params.Get("market"))
		// When no time is specified, neither timestamp nor order should be set
		assert.Empty(t, params.Get("timestamp"))
		assert.Empty(t, params.Get("order"))
	})
}

func TestExchange_buildFromIdTradesRequest(t *testing.T) {
	ex := New("key", "secret", "")

	t.Run("lastTradeIsAfterStartTime_UseFromId", func(t *testing.T) {
		startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		lastTradeTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

		lastTrade := &types.Trade{
			ID:   12345,
			Time: types.Time(lastTradeTime),
		}

		options := &types.TradeQueryOptions{
			LastTradeID: 12345,
			StartTime:   &startTime,
		}

		req := ex.buildFromIdTradesRequest("BTCUSDT", options, lastTrade)

		// Check query parameters
		params, err := req.GetParametersQuery()
		assert.NoError(t, err)
		assert.Equal(t, "btcusdt", params.Get("market"))
		assert.Equal(t, "asc", params.Get("order"))
		assert.Equal(t, "12345", params.Get("from_id"))
		// Timestamp should not be set when using from_id
		assert.Empty(t, params.Get("timestamp"))
	})

	t.Run("lastTradeIsBeforeStartTime_FallbackToTimeRange", func(t *testing.T) {
		startTime := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
		lastTradeTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		lastTrade := &types.Trade{
			ID:   12345,
			Time: types.Time(lastTradeTime),
		}

		options := &types.TradeQueryOptions{
			LastTradeID: 12345,
			StartTime:   &startTime,
		}

		req := ex.buildFromIdTradesRequest("BTCUSDT", options, lastTrade)

		// Check query parameters
		params, err := req.GetParametersQuery()
		assert.NoError(t, err)
		assert.Equal(t, "btcusdt", params.Get("market"))
		assert.Equal(t, "asc", params.Get("order"))
		// Timestamp is in milliseconds
		assert.Equal(t, strconv.FormatInt(startTime.UnixMilli(), 10), params.Get("timestamp"))
		// from_id should not be set when falling back to time range
		assert.Empty(t, params.Get("from_id"))
	})

	t.Run("lastTradeIsNilWithStartTime_UseStartTime", func(t *testing.T) {
		startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		options := &types.TradeQueryOptions{
			LastTradeID: 12345,
			StartTime:   &startTime,
		}

		req := ex.buildFromIdTradesRequest("BTCUSDT", options, nil)

		// Check query parameters
		params, err := req.GetParametersQuery()
		assert.NoError(t, err)
		assert.Equal(t, "btcusdt", params.Get("market"))
		assert.Equal(t, "asc", params.Get("order"))
		// Timestamp is in milliseconds
		assert.Equal(t, strconv.FormatInt(startTime.UnixMilli(), 10), params.Get("timestamp"))
		assert.Empty(t, params.Get("from_id"))
	})

	t.Run("lastTradeIsNilWithEndTime_UseEndTimeDesc", func(t *testing.T) {
		endTime := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

		options := &types.TradeQueryOptions{
			LastTradeID: 12345,
			EndTime:     &endTime,
		}

		req := ex.buildFromIdTradesRequest("BTCUSDT", options, nil)

		// Check query parameters
		params, err := req.GetParametersQuery()
		assert.NoError(t, err)
		assert.Equal(t, "btcusdt", params.Get("market"))
		assert.Equal(t, "desc", params.Get("order"))
		// Timestamp is in milliseconds
		assert.Equal(t, strconv.FormatInt(endTime.UnixMilli(), 10), params.Get("timestamp"))
		assert.Empty(t, params.Get("from_id"))
	})

	t.Run("lastTradeIsNilAndNoTimeSpecified", func(t *testing.T) {
		options := &types.TradeQueryOptions{
			LastTradeID: 12345,
		}

		req := ex.buildFromIdTradesRequest("BTCUSDT", options, nil)

		// Check query parameters
		params, err := req.GetParametersQuery()
		assert.NoError(t, err)
		assert.Equal(t, "btcusdt", params.Get("market"))
		assert.Empty(t, params.Get("timestamp"))
		assert.Empty(t, params.Get("from_id"))
		assert.Empty(t, params.Get("order"))
	})

	t.Run("marginWalletCheckWalletType", func(t *testing.T) {
		ex.MarginSettings.IsMargin = true
		defer func() { ex.MarginSettings.IsMargin = false }()

		startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		lastTradeTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

		lastTrade := &types.Trade{
			ID:   12345,
			Time: types.Time(lastTradeTime),
		}

		options := &types.TradeQueryOptions{
			LastTradeID: 12345,
			StartTime:   &startTime,
		}

		req := ex.buildFromIdTradesRequest("BTCUSDT", options, lastTrade)

		// Check slug parameters (wallet type)
		slugParams, err := req.GetSlugParameters()
		assert.NoError(t, err)
		// WalletType is a custom type, so compare as string
		assert.Equal(t, "m", string(slugParams["walletType"].(maxapi.WalletType)))
	})
}
