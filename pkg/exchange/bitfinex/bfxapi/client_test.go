package bfxapi

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/testutil"
)

const AlwaysRecord = true
const RecordIfFileNotFound = false

func RunHttpTestWithRecorder(t *testing.T, client *http.Client, recordFile string) (bool, func()) {
	mockTransport := &httptesting.MockTransport{}
	recorder := httptesting.NewRecorder(http.DefaultTransport)

	_, fErr := os.Stat(recordFile)
	notFound := fErr != nil && os.IsNotExist(fErr)
	shouldRecord := RecordIfFileNotFound && notFound

	if os.Getenv("TEST_HTTP_RECORD") == "1" || shouldRecord || AlwaysRecord {
		client.Transport = recorder
		return true, func() {
			if err := recorder.Save(recordFile); err != nil {
				t.Errorf("failed to save recorded requests: %v", err)
			}
		}
	} else {
		if err := recorder.Load(recordFile); err != nil {
			t.Fatalf("failed to load recorded requests: %v", err)
		}

		if err := mockTransport.LoadFromRecorder(recorder); err != nil {
			t.Fatalf("failed to load recordings: %v", err)
		}

		client.Transport = mockTransport
		return false, func() {}
	}
}

func TestClient_privateApis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient()

	isRecording, saveRecord := RunHttpTestWithRecorder(t, client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITFINEX")
	if ok {
		client.Auth(key, secret)
	}

	if isRecording && !ok {
		t.Skipf("BITFINEX api key is not configured, skipping integration test")
	}

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
}

func TestClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient()

	isRecording, saveRecord := RunHttpTestWithRecorder(t, client.HttpClient, "testdata/"+t.Name()+".json")
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
