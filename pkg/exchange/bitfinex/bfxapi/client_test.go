package bfxapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func TestClient(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITFINEX")
	if !ok {
		t.Skipf("BITFINEX api key is not configured, skipping integration test")
	}

	client := NewClient()
	client.Auth(key, secret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	t.Run("GetWalletsRequest", func(t *testing.T) {
		req := client.NewGetWalletsRequest()
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("wallets response: %+v", resp)
		assert.NotEmpty(t, resp, "expected non-empty wallets response")
	})

	t.Run("GetBookRequest", func(t *testing.T) {
		req := client.NewGetBookRequest()
		req.Symbol("tBTCUSD")
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("book response: %+v", resp)
		assert.NotEmpty(t, resp.BookEntries, "expected non-empty book entries")
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

		defer func() {
			// cancel the submitted order to clean up
			cancelReq := client.NewCancelOrderRequest()
			cancelReq.OrderID(orderID)
			cancelResp, err := cancelReq.Do(ctx)
			assert.NoError(t, err)
			t.Logf("cancel order response: %+v", cancelResp)
		}()

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
}
