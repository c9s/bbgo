package bfxapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/testutil"
)

func TestClient_orderApis(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient()

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITFINEX")
	if ok {
		client.Auth(key, secret)
	}

	if isRecording && !ok {
		t.Skipf("BITFINEX api key is not configured, skipping integration test")
	}

	t.Run("RetrieveOrderRequest", func(t *testing.T) {
		req := client.NewRetrieveOrderRequest()
		// Optionally, you can add filters like ID, GID, CID, etc.
		// req.AddId(123456789) // example order ID
		resp, err := req.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("active order response: %+v", resp)
			for _, order := range resp.Orders {
				t.Logf("active order: %+v", order)
			}
		}
	})

	t.Run("SubmitOrderRequest", func(t *testing.T) {
		// submit a small test order, e.g. limit order for BTCUSD
		req := client.NewSubmitOrderRequest()
		req.Symbol("tBTCUST")
		req.Amount("0.001")  // small amount for test
		req.Price("10500.0") // far from market price to avoid execution
		req.OrderType(OrderTypeExchangeLimit)

		resp, err := req.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("submit order response: %+v", resp)
			assert.NotNil(t, resp.Data, "expected order data in response")
		} else {
			return
		}

		// retrieve the submitted order by ID
		orderID := resp.Data[0].OrderID

		t.Cleanup(func() {
			t.Logf("test case %s cleaning up", t.Name())
			// cancel the submitted order to clean up
			cancelReq := client.NewCancelOrderRequest()
			cancelReq.OrderID(orderID)
			cancelResp, err := cancelReq.Do(ctx)
			assert.NoError(t, err)
			t.Logf("cancel order response: %+v", cancelResp)
		})

		retrieveReq := client.NewRetrieveOrderRequest()
		retrieveReq.AddId(orderID)
		retrieveResp, err := retrieveReq.Do(ctx)
		if assert.NoError(t, err) {
			assert.NotEmpty(t, retrieveResp.Orders, "expected non-empty orders response")
			for _, order := range retrieveResp.Orders {
				t.Logf("retrieved order: %+v", order)
			}
		}
	})

	t.Run("GetWalletsRequest", func(t *testing.T) {
		req := client.NewGetWalletsRequest()
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("wallets response: %+v", resp)
		assert.NotEmpty(t, resp, "expected non-empty wallets response")
	})

	t.Run("GetOrderHistoryRequest", func(t *testing.T) {
		req := client.NewGetOrderHistoryRequest()
		req.Limit(5) // limit to 5 orders for testing
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("order history response: %+v", resp)

		if assert.NotEmpty(t, resp, "expected non-empty order history") {
			for _, order := range resp {
				t.Logf("order: %+v", order.String())
			}
		}
	})

	t.Run("GetOrderHistoryBySymbolRequest", func(t *testing.T) {
		req := client.NewGetOrderHistoryBySymbolRequest()
		req.Symbol("tBTCUST")
		req.Limit(10) // limit to 5 orders for testing

		resp, err := req.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("order history by symbol response: %+v", resp)
			if assert.NotEmpty(t, resp, "expected non-empty order history by symbol") {
				for _, order := range resp {
					t.Log(order.String())
				}
			}
		}
	})

	// Test GetOrderTradesRequest
	t.Run("GetOrderTradesRequest", func(t *testing.T) {
		submitOrder := func() {
			submitOrderReq := client.NewSubmitOrderRequest()
			submitOrderReq.Symbol("tBTCUST").Amount("0.001").OrderType(OrderTypeExchangeMarket)
			submitOrderResp, err := submitOrderReq.Do(ctx)
			if assert.NoError(t, err) {
				t.Logf("submit order response: %+v", submitOrderResp)
				assert.NotNil(t, submitOrderResp.Data, "expected order data in response")
			} else {
				return
			}
		}

		var orders []Order

		getOrderHistory := func() {
			var err error
			// For testing, we need a valid symbol and order ID. Use order history to get one.
			orderHistoryReq := client.NewGetOrderHistoryRequest()
			orderHistoryReq.Limit(10)
			orders, err = orderHistoryReq.Do(ctx)
			if assert.NoError(t, err) {
				if len(orders) == 0 {
					t.Skip("no orders found for order trades test")
				} else {
					t.Logf("found %d orders in history", len(orders))
				}
			}

			assert.NotEmpty(t, orders, "expected non-empty order history")
		}

		getOrderHistory()

		if len(orders) == 0 {
			submitOrder()
			getOrderHistory()
		}

		foundExecOrder := false
		order := orders[0]

		// find filled order
		findExecOrder := func() {
			for _, o := range orders {
				if o.Amount.Compare(o.AmountOrig) != 0 {
					order = o
					foundExecOrder = true
					return
				}
			}
		}
		findExecOrder()

		if !foundExecOrder {
			submitOrder()
			getOrderHistory()
			findExecOrder()
		}

		t.Logf("using order: %+v for trades test", order)

		if order.OrderID == 0 || order.Symbol == "" {
			t.Skip("no valid order found for order trades test")
		}

		if !order.Amount.IsZero() {
			t.Skipf("order %d has zero amount, skipping order trades test", order.OrderID)
		}

		tradesReq := client.NewGetOrderTradesRequest()
		tradesReq.Symbol(order.Symbol)
		tradesReq.Id(order.OrderID)
		trades, err := tradesReq.Do(ctx)
		assert.NoError(t, err)
		t.Logf("order trades response: %+v", trades)
		if assert.NotEmpty(t, trades, "expected non-empty order trades response") {
			for _, trade := range trades {
				t.Logf("trade: %+v", trade)
			}
		}
	})

	t.Run("GetTradeHistoryBySymbolRequest", func(t *testing.T) {
		req := client.NewGetTradeHistoryBySymbolRequest()
		req.Symbol("tBTCUST")
		req.Limit(5) // limit to 5 trades for testing

		resp, err := req.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("trade history response: %+v", resp)
			if assert.NotEmpty(t, resp, "expected non-empty trade history") {
				for _, trade := range resp {
					t.Logf("trade: %+v", trade)
				}
			}
		}
	})

	t.Run("GetTradeHistoryRequest", func(t *testing.T) {
		req := client.NewGetTradeHistoryRequest()
		req.Limit(5) // limit to 5 trades for testing

		resp, err := req.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("trade history response: %+v", resp)
			if assert.NotEmpty(t, resp, "expected non-empty trade history") {
				for _, trade := range resp {
					t.Logf("trade: %+v", trade)
				}
			}
		}
	})
}

func TestClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient()

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITFINEX")
	if ok {
		client.Auth(key, secret)
	}

	if isRecording && !ok {
		t.Skipf("BITFINEX api key is not configured, skipping integration test")
	}

	t.Run("GetTickerRequest", func(t *testing.T) {
		req := client.NewGetTickerRequest()
		req.Symbol("tBTCUSD")
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("ticker response: %+v", resp)
	})

	t.Run("GetCandlesRequest", func(t *testing.T) {
		req := client.NewGetCandlesRequest()
		req.Candle("trade:1m:tBTCUSD")
		req.Section("hist")

		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("candles response: %+v", resp)
	})

	t.Run("GetPairConfigRequest", func(t *testing.T) {
		req := client.NewGetPairConfigRequest()
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("pair config response: %+v", resp)

		for _, pair := range resp.Pairs {
			t.Logf("pair: %s, min order size: %s, max order size: %s, initial margin: %s, min margin: %s",
				pair.Pair,
				pair.MinOrderSize.String(),
				pair.MaxOrderSize.String(),
				pair.InitialMargin.String(),
				pair.MinMargin.String())
		}
	})

	t.Run("GetTickersRequest", func(t *testing.T) {
		req := client.NewGetTickersRequest()
		req.Symbols("ALL")
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("tickers response: %+v", resp)
	})

	t.Run("GetBookRequest", func(t *testing.T) {
		req := client.NewGetBookRequest()
		req.Symbol("tBTCUSD")
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("book response: %+v", resp)
		assert.NotEmpty(t, resp.BookEntries, "expected non-empty book entries")
	})
}

func TestClient_fundingApis(t *testing.T) {
	// Enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient()

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITFINEX")
	if ok {
		client.Auth(key, secret)
	}

	if isRecording && !ok {
		t.Skipf("BITFINEX api key is not configured, skipping integration test")
	}

	t.Run("SubmitFundingOfferRequest", func(t *testing.T) {
		req := client.NewSubmitFundingOfferRequest()
		req.Symbol("fUST").
			Amount("150").
			Rate("0.0002").
			Period(2).
			OfferType(FundingOfferTypeLimit)

		resp, err := req.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("submit funding offer response: %+v", resp)
			assert.NotNil(t, resp.FundingOffer.ID, "expected funding offer ID in response")

			id := resp.FundingOffer.ID
			t.Logf("canceling funding offer with ID: %d", id)

			cancelReq := client.NewCancelFundingOfferRequest()
			cancelReq.Id(id)
			cancelResp, err := cancelReq.Do(ctx)
			assert.NoError(t, err)
			t.Logf("cancel funding offer response: %+v", cancelResp)
			assert.Equal(t, id, cancelResp.FundingOffer.ID, "expected matching funding offer ID in cancel response")
		}
	})

	t.Run("GetActiveFundingOffersRequest", func(t *testing.T) {
		req := client.NewGetActiveFundingOffersRequest()
		resp, err := req.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("active funding offers response: %+v", resp)
			for _, offer := range resp {
				cancelReq := client.NewCancelFundingOfferRequest()
				cancelReq.Id(offer.ID)
				cancelResp, err := cancelReq.Do(ctx)
				assert.NoError(t, err)
				t.Logf("cancel funding offer response: %+v", cancelResp)
			}
		}
	})
}
