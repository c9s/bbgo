package max

import (
	"context"
	"sort"
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
	t.SkipNow() // skip flaky test for now
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

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, ex.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

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

func TestExchange_QueryWithdrawHistory(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	since, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	until, err := time.Parse(time.RFC3339, "2026-02-28T00:00:00Z")

	withdraws, err := e.QueryWithdrawHistory(context.Background(), "", since, until)
	if assert.NoError(t, err) {
		assert.NotNil(t, withdraws)
		t.Logf("found %d withdraws", len(withdraws))

		for _, withdraw := range withdraws {
			assert.NotEmpty(t, withdraw.Asset)
			assert.NotEmpty(t, withdraw.Status)
			t.Logf("withdraw: %+v", withdraw)
		}
	}
}

func TestExchange_QueryDepositHistory(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	since, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	until, err := time.Parse(time.RFC3339, "2026-02-28T00:00:00Z")

	deposits, err := e.QueryDepositHistory(context.Background(), "", since, until)
	if assert.NoError(t, err) {
		assert.NotNil(t, deposits)
		t.Logf("found %d deposits", len(deposits))

		for _, deposit := range deposits {
			assert.NotEmpty(t, deposit.Asset)
			assert.NotEmpty(t, deposit.Status)
			t.Logf("deposit: %+v", deposit)
		}
	}
}

func TestExchange_QueryClosedOrders(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	since, err := time.Parse(time.RFC3339, "2026-02-01T00:00:00Z")
	if !assert.NoError(t, err) {
		return
	}

	until, err := time.Parse(time.RFC3339, "2026-02-10T00:00:00Z")
	if !assert.NoError(t, err) {
		return
	}

	ctx := context.Background()
	orders, err := e.QueryClosedOrders(ctx, "BTCUSDT", since, until, 0)
	assert.NoError(t, err)
	assert.NotNil(t, orders)
	t.Logf("found %d closed orders", len(orders))

	for _, order := range orders {
		assert.NotEmpty(t, order.Symbol)
		assert.NotZero(t, order.OrderID)
		assert.NotEmpty(t, order.Status)
		assert.True(t, !order.CreationTime.Time().Before(since))
		assert.True(t, !order.CreationTime.Time().After(until))
		t.Logf("closed order: OrderID=%d Symbol=%s Status=%s Side=%s Price=%s Quantity=%s CreationTime=%s",
			order.OrderID, order.Symbol, order.Status, order.Side, order.Price, order.Quantity, order.CreationTime)
	}

	sort.Slice(orders, func(i, j int) bool {
		return orders[i].OrderID < orders[j].OrderID
	})
	lastOrderID := orders[len(orders)-1].OrderID
	orders, err = e.QueryClosedOrders(
		ctx,
		"BTCUSDT",
		time.Time{},
		time.Time{},
		lastOrderID,
	)
	assert.NoError(t, err)
}

func TestExchange_QueryClosedOrdersDesc(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	since, err := time.Parse(time.RFC3339, "2026-02-01T00:00:00Z")
	if !assert.NoError(t, err) {
		return
	}

	until, err := time.Parse(time.RFC3339, "2026-02-10T00:00:00Z")
	if !assert.NoError(t, err) {
		return
	}

	orders, err := e.QueryClosedOrdersDesc(context.Background(), "BTCUSDT", since, until, 0)
	if assert.NoError(t, err) {
		assert.NotNil(t, orders)
		t.Logf("found %d closed orders (desc)", len(orders))

		for _, order := range orders {
			assert.NotEmpty(t, order.Symbol)
			assert.NotZero(t, order.OrderID)
			assert.NotEmpty(t, order.Status)
			assert.True(t, !order.CreationTime.Time().Before(since))
			assert.True(t, !order.CreationTime.Time().After(until))
			t.Logf("closed order (desc): OrderID=%d Symbol=%s Status=%s Side=%s Price=%s Quantity=%s CreationTime=%s",
				order.OrderID, order.Symbol, order.Status, order.Side, order.Price, order.Quantity, order.CreationTime)
		}
	}
}

func TestExchange_QueryOrder(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	orderID := uint64(2492647274)

	order, err := e.QueryOrder(context.Background(), types.OrderQuery{
		OrderID: strconv.FormatUint(orderID, 10),
	})

	if assert.NoError(t, err) {
		assert.NotNil(t, order)
		assert.Equal(t, orderID, order.OrderID)
		assert.NotEmpty(t, order.Symbol)
		assert.NotEmpty(t, order.Status)
		assert.NotEmpty(t, order.Side)
		assert.False(t, order.Price.IsZero())
		assert.False(t, order.Quantity.IsZero())
		assert.False(t, order.CreationTime.Time().IsZero())
		t.Logf("order: OrderID=%d Symbol=%s Status=%s Side=%s Type=%s Price=%s Quantity=%s ExecutedQuantity=%s CreationTime=%s",
			order.OrderID, order.Symbol, order.Status, order.Side, order.Type, order.Price, order.Quantity, order.ExecutedQuantity, order.CreationTime)
	}
}

func TestExchange_QueryOrderTrades(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	orderID := uint64(2492647274)

	trades, err := e.QueryOrderTrades(context.Background(), types.OrderQuery{
		OrderID: strconv.FormatUint(orderID, 10),
	})

	if assert.NoError(t, err) {
		assert.NotNil(t, trades)
		t.Logf("found %d trades for order %d", len(trades), orderID)

		for _, trade := range trades {
			assert.Equal(t, orderID, trade.OrderID)
			assert.NotEmpty(t, trade.Symbol)
			assert.NotZero(t, trade.ID)
			assert.NotEmpty(t, trade.Side)
			assert.False(t, trade.Price.IsZero())
			assert.False(t, trade.Quantity.IsZero())
			assert.False(t, trade.QuoteQuantity.IsZero())
			assert.False(t, trade.Time.Time().IsZero())
			t.Logf("trade: TradeID=%d OrderID=%d Symbol=%s Side=%s Price=%s Quantity=%s QuoteQuantity=%s Fee=%s FeeCurrency=%s Time=%s",
				trade.ID, trade.OrderID, trade.Symbol, trade.Side, trade.Price, trade.Quantity, trade.QuoteQuantity, trade.Fee, trade.FeeCurrency, trade.Time)
		}
	}
}

func TestExchange_QueryAccount(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	t.Run("QueryAccount", func(t *testing.T) {
		e := New(key, secret, "")

		isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
		defer saveRecord()

		if isRecording && !ok {
			t.Skipf("MAX api key is not configured, skipping integration test")
		}

		account, err := e.QueryAccount(context.Background())
		if assert.NoError(t, err) {
			assert.NotNil(t, account)
			assert.Equal(t, types.AccountTypeSpot, account.AccountType)
			assert.True(t, account.HasFeeRate)
			assert.False(t, account.MakerFeeRate.IsZero())
			assert.False(t, account.TakerFeeRate.IsZero())
			balance, ok := account.Balance("USDT")
			assert.True(t, ok)
			assert.NotNil(t, balance)
			t.Logf("account: AccountType=%s MakerFeeRate=%s TakerFeeRate=%s",
				account.AccountType, account.MakerFeeRate, account.TakerFeeRate)
		}
	})

	t.Run("QuerySpotAccount", func(t *testing.T) {
		e := New(key, secret, "")

		isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
		defer saveRecord()

		if isRecording && !ok {
			t.Skipf("MAX api key is not configured, skipping integration test")
		}

		account, err := e.QuerySpotAccount(context.Background())
		if assert.NoError(t, err) {
			assert.NotNil(t, account)
			assert.Equal(t, types.AccountTypeSpot, account.AccountType)
			assert.True(t, account.HasFeeRate)
			assert.False(t, account.MakerFeeRate.IsZero())
			assert.False(t, account.TakerFeeRate.IsZero())
			balance, ok := account.Balance("USDT")
			assert.True(t, ok)
			assert.NotNil(t, balance)
			t.Logf("account: AccountType=%s MakerFeeRate=%s TakerFeeRate=%s",
				account.AccountType, account.MakerFeeRate, account.TakerFeeRate)
		}
	})
}

func TestExchange_QueryKLines(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	since, err := time.Parse(time.RFC3339, "2026-02-01T00:00:00Z")
	if !assert.NoError(t, err) {
		return
	}

	klines, err := e.QueryKLines(context.Background(), "BTCUSDT", types.Interval1h, types.KLineQueryOptions{
		StartTime: &since,
		Limit:     100,
	})

	if assert.NoError(t, err) {
		assert.NotNil(t, klines)
		assert.NotEmpty(t, klines)
		t.Logf("found %d klines", len(klines))

		for _, kline := range klines {
			assert.Equal(t, "BTCUSDT", kline.Symbol)
			assert.Equal(t, types.Interval1h, kline.Interval)
			assert.False(t, kline.Open.IsZero())
			assert.False(t, kline.Close.IsZero())
			assert.False(t, kline.High.IsZero())
			assert.False(t, kline.Low.IsZero())
			assert.False(t, kline.StartTime.Time().IsZero())
			assert.False(t, kline.EndTime.Time().IsZero())
			assert.True(t, kline.StartTime.Time().Equal(since) || kline.StartTime.Time().After(since))
			t.Logf("kline: Symbol=%s Interval=%s Open=%s Close=%s High=%s Low=%s Volume=%s StartTime=%s",
				kline.Symbol, kline.Interval, kline.Open, kline.Close, kline.High, kline.Low, kline.Volume, kline.StartTime)
		}
	}
}

func TestExchange_QueryDepth(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	orderBook, updateID, err := e.QueryDepth(context.Background(), "BTCUSDT", 50)

	if assert.NoError(t, err) {
		assert.Equal(t, "BTCUSDT", orderBook.Symbol)
		assert.NotEmpty(t, orderBook.Bids)
		assert.NotEmpty(t, orderBook.Asks)
		assert.NotZero(t, updateID)
		t.Logf("orderBook: Symbol=%s Bids=%d Asks=%d UpdateID=%d",
			orderBook.Symbol, len(orderBook.Bids), len(orderBook.Asks), updateID)

		// Verify bids are sorted in descending order (highest price first)
		for i := 0; i < len(orderBook.Bids)-1; i++ {
			assert.True(t, orderBook.Bids[i].Price.Compare(orderBook.Bids[i+1].Price) >= 0,
				"bids should be sorted in descending order")
		}

		// Verify asks are sorted in ascending order (lowest price first)
		for i := 0; i < len(orderBook.Asks)-1; i++ {
			assert.True(t, orderBook.Asks[i].Price.Compare(orderBook.Asks[i+1].Price) <= 0,
				"asks should be sorted in ascending order")
		}

		// Log first few bids and asks
		for i := 0; i < 5 && i < len(orderBook.Bids); i++ {
			t.Logf("bid[%d]: Price=%s Volume=%s", i, orderBook.Bids[i].Price, orderBook.Bids[i].Volume)
		}
		for i := 0; i < 5 && i < len(orderBook.Asks); i++ {
			t.Logf("ask[%d]: Price=%s Volume=%s", i, orderBook.Asks[i].Price, orderBook.Asks[i].Volume)
		}
	}
}

func TestExchange_QueryTrades(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	e := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, e.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	startTime, err := time.Parse("2006-01-02", "2026-02-01")
	if !assert.NoError(t, err) {
		return
	}

	endTime, err := time.Parse("2006-01-02", "2026-02-10")
	if !assert.NoError(t, err) {
		return
	}

	ctx := context.Background()
	trades, err := e.QueryTrades(ctx, "BTCUSDT", &types.TradeQueryOptions{
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     100,
	})
	assert.NoError(t, err)
	assert.NotNil(t, trades)
	t.Logf("found %d trades", len(trades))

	for _, trade := range trades {
		assert.Equal(t, "BTCUSDT", trade.Symbol)
		assert.Equal(t, types.ExchangeMax, trade.Exchange)
		assert.False(t, trade.Price.IsZero())
		assert.False(t, trade.Quantity.IsZero())
		assert.NotZero(t, trade.ID)
		assert.NotZero(t, trade.OrderID)
		assert.True(t, trade.Time.Time().Equal(startTime) || trade.Time.Time().After(startTime))
		assert.True(t, trade.Time.Time().Equal(endTime) || trade.Time.Time().Before(endTime))
		assert.Contains(t, []types.SideType{types.SideTypeBuy, types.SideTypeSell}, trade.Side)

		t.Logf("trade: ID=%d OrderID=%d Symbol=%s Side=%s Price=%s Quantity=%s Time=%s Fee=%s FeeCurrency=%s",
			trade.ID, trade.OrderID, trade.Symbol, trade.Side, trade.Price, trade.Quantity, trade.Time, trade.Fee, trade.FeeCurrency)
	}

	// Verify trades are sorted in ascending order by time
	for i := 0; i < len(trades)-1; i++ {
		assert.True(t, trades[i].Time.Time().Before(trades[i+1].Time.Time()) ||
			trades[i].Time.Time().Equal(trades[i+1].Time.Time()),
			"trades should be sorted in ascending order by time")
	}

	lastTradeID := trades[len(trades)-1].ID
	trades, err = e.QueryTrades(ctx, "BTCUSDT", &types.TradeQueryOptions{
		LastTradeID: lastTradeID,
	})
	assert.NotEmpty(t, trades)
	assert.NoError(t, err)
}

func TestExchange_CancelOrdersBySymbolSide(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	ex := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, ex.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	// Query markets to get market info
	markets, err := ex.QueryMarkets(ctx)
	if !assert.NoError(t, err) {
		return
	}

	market, ok := markets["BTCUSDT"]
	if !assert.True(t, ok, "BTCUSDT market not found") {
		return
	}

	// Create a limit order to buy 0.1 BTC at the price of 1000 USDT
	submitOrder := types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Price:    Number(1000),
		Quantity: Number(0.1),
		Market:   market,
	}

	order, err := ex.SubmitOrder(ctx, submitOrder)
	if !assert.NoError(t, err) {
		return
	}

	assert.NotNil(t, order)
	assert.Equal(t, "BTCUSDT", order.Symbol)
	assert.Equal(t, types.SideTypeBuy, order.Side)
	assert.Equal(t, types.OrderTypeLimit, order.Type)
	assert.Equal(t, Number(1000), order.Price)
	assert.Equal(t, Number(0.1), order.Quantity)
	assert.NotZero(t, order.OrderID)
	t.Logf("created order: %+v", order)

	// Cancel the order by CancelOrdersBySymbolSide
	canceledOrders, err := ex.CancelOrdersBySymbolSide(ctx, "BTCUSDT", types.SideTypeBuy)
	if !assert.NoError(t, err) {
		return
	}

	// Verify the response
	assert.NotEmpty(t, canceledOrders)
	t.Logf("canceled %d orders", len(canceledOrders))

	// Verify that our order is in the canceled orders
	found := false
	for _, canceledOrder := range canceledOrders {
		if canceledOrder.OrderID == order.OrderID {
			found = true
			assert.Equal(t, "BTCUSDT", canceledOrder.Symbol)
			assert.Equal(t, types.SideTypeBuy, canceledOrder.Side)
			assert.Equal(t, types.OrderTypeLimit, canceledOrder.Type)
			t.Logf("found canceled order: %+v", canceledOrder)
			break
		}
	}

	assert.True(t, found, "canceled order should be in the response")
}

func TestExchange_CancelOrdersBySymbol(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	ex := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, ex.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	// Query markets to get market info
	markets, err := ex.QueryMarkets(ctx)
	if !assert.NoError(t, err) {
		return
	}

	market, ok := markets["BTCUSDT"]
	if !assert.True(t, ok, "BTCUSDT market not found") {
		return
	}

	// Create a limit order to buy 0.1 BTC at the price of 1000 USDT
	submitOrder := types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Price:    Number(1000),
		Quantity: Number(0.1),
		Market:   market,
	}

	order, err := ex.SubmitOrder(ctx, submitOrder)
	if !assert.NoError(t, err) {
		return
	}

	assert.NotNil(t, order)
	assert.Equal(t, "BTCUSDT", order.Symbol)
	assert.Equal(t, types.SideTypeBuy, order.Side)
	assert.Equal(t, types.OrderTypeLimit, order.Type)
	assert.Equal(t, Number(1000), order.Price)
	assert.Equal(t, Number(0.1), order.Quantity)
	assert.NotZero(t, order.OrderID)
	t.Logf("created order: %+v", order)

	// Cancel all orders by symbol
	canceledOrders, err := ex.CancelOrdersBySymbol(ctx, "BTCUSDT")
	if !assert.NoError(t, err) {
		return
	}

	// Verify the response
	assert.NotEmpty(t, canceledOrders)
	t.Logf("canceled %d orders", len(canceledOrders))

	// Verify that our order is in the canceled orders
	found := false
	for _, canceledOrder := range canceledOrders {
		if canceledOrder.OrderID == order.OrderID {
			found = true
			assert.Equal(t, "BTCUSDT", canceledOrder.Symbol)
			assert.Equal(t, types.SideTypeBuy, canceledOrder.Side)
			assert.Equal(t, types.OrderTypeLimit, canceledOrder.Type)
			t.Logf("found canceled order: %+v", canceledOrder)
			break
		}
	}

	assert.True(t, found, "canceled order should be in the response")
}

func TestExchange_QueryOpenOrders(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ex := New(key, secret, "")

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, ex.v3client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	// Call QueryOpenOrders
	openOrders, err := ex.QueryOpenOrders(ctx, "BTCUSDT")
	assert.NoError(t, err)
	t.Logf("open orders count: %d", len(openOrders))

	// Verify each order if there are any
	for _, order := range openOrders {
		assert.NotZero(t, order.OrderID)
		assert.Equal(t, "BTCUSDT", order.Symbol)
		assert.True(t, order.Status == types.OrderStatusNew || order.Status == types.OrderStatusPartiallyFilled)
		t.Logf("open order: %+v", order)
	}
}
