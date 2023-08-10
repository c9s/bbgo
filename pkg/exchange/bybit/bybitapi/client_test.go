package bybitapi

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func getTestClientOrSkip(t *testing.T) *RestClient {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BYBIT")
	if !ok {
		t.Skip("BYBIT_* env vars are not configured")
		return nil
	}

	client, err := NewClient()
	assert.NoError(t, err)
	client.Auth(key, secret)
	return client
}

func TestClient(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	t.Run("GetAccountInfoRequest", func(t *testing.T) {
		req := client.NewGetAccountRequest()
		accountInfo, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("accountInfo: %+v", accountInfo)
	})

	t.Run("GetInstrumentsInfoRequest", func(t *testing.T) {
		req := client.NewGetInstrumentsInfoRequest()
		instrumentsInfo, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("instrumentsInfo: %+v", instrumentsInfo)
	})

	t.Run("GetTicker", func(t *testing.T) {
		req := client.NewGetTickersRequest()
		apiResp, err := req.Symbol("BTCUSDT").Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %+v", apiResp)

		req = client.NewGetTickersRequest()
		tickers, err := req.Symbol("BTCUSDT").DoWithResponseTime(ctx)
		assert.NoError(t, err)
		t.Logf("tickers: %+v", tickers)
	})

	t.Run("GetOpenOrderRequest", func(t *testing.T) {
		cursor := ""
		for {
			req := client.NewGetOpenOrderRequest().Limit(1)
			if len(cursor) != 0 {
				req = req.Cursor(cursor)
			}
			openOrders, err := req.Do(ctx)
			assert.NoError(t, err)

			for _, o := range openOrders.List {
				t.Logf("openOrders: %+v", o)
			}
			if len(openOrders.NextPageCursor) == 0 {
				break
			}
			cursor = openOrders.NextPageCursor
		}
	})

	t.Run("PlaceOrderRequest", func(t *testing.T) {
		req := client.NewPlaceOrderRequest().
			Symbol("DOTUSDT").
			Side(SideBuy).
			OrderType(OrderTypeLimit).
			Qty("1").
			Price("4.6").
			OrderLinkId(uuid.NewString()).
			TimeInForce(TimeInForceGTC)
		apiResp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %+v", apiResp)

		_, err = strconv.ParseUint(apiResp.OrderId, 10, 64)
		assert.NoError(t, err)

		ordersResp, err := client.NewGetOpenOrderRequest().OrderLinkId(apiResp.OrderLinkId).Do(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(ordersResp.List), 1)
		t.Logf("apiResp: %+v", ordersResp.List[0])
	})

	t.Run("CancelOrderRequest", func(t *testing.T) {
		req := client.NewPlaceOrderRequest().
			Symbol("DOTUSDT").
			Side(SideBuy).
			OrderType(OrderTypeLimit).
			Qty("1").
			Price("4.6").
			OrderLinkId(uuid.NewString()).
			TimeInForce(TimeInForceGTC)
		apiResp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %+v", apiResp)

		ordersResp, err := client.NewGetOpenOrderRequest().OrderLinkId(apiResp.OrderLinkId).Do(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(ordersResp.List), 1)
		t.Logf("apiResp: %+v", ordersResp.List[0])

		cancelReq := client.NewCancelOrderRequest().
			Symbol("DOTUSDT").
			OrderLinkId(apiResp.OrderLinkId)
		cancelResp, err := cancelReq.Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %+v", cancelResp)

		ordersResp, err = client.NewGetOpenOrderRequest().OrderLinkId(apiResp.OrderLinkId).Do(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(ordersResp.List), 1)
		assert.Equal(t, ordersResp.List[0].OrderStatus, OrderStatusCancelled)
		t.Logf("apiResp: %+v", ordersResp.List[0])
	})

	t.Run("GetOrderHistoriesRequest", func(t *testing.T) {
		req := client.NewPlaceOrderRequest().
			Symbol("DOTUSDT").
			Side(SideBuy).
			OrderType(OrderTypeLimit).
			Qty("1").
			Price("4.6").
			OrderLinkId(uuid.NewString()).
			TimeInForce(TimeInForceGTC)
		apiResp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %+v", apiResp)

		ordersResp, err := client.NewGetOpenOrderRequest().OrderLinkId(apiResp.OrderLinkId).Do(ctx)
		assert.NoError(t, err)
		assert.Equal(t, len(ordersResp.List), 1)
		t.Logf("apiResp: %+v", ordersResp.List[0])

		orderResp, err := client.NewGetOrderHistoriesRequest().Symbol("DOTUSDT").Cursor("0").Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %#v", orderResp)
	})

	t.Run("GetWalletBalancesRequest", func(t *testing.T) {
		req := client.NewGetWalletBalancesRequest().Coin("BTC")
		apiResp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %+v", apiResp)
	})

	t.Run("GetKLinesRequest", func(t *testing.T) {
		startTime := time.Date(2023, 8, 8, 9, 28, 0, 0, time.UTC)
		endTime := time.Date(2023, 8, 8, 9, 45, 0, 0, time.UTC)
		req := client.NewGetKLinesRequest().
			Symbol("BTCUSDT").Interval("15").StartTime(startTime).EndTime(endTime)
		apiResp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %+v", apiResp.List)
	})

	t.Run("GetFeeRatesRequest", func(t *testing.T) {
		req := client.NewGetFeeRatesRequest()
		apiResp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("apiResp: %+v", apiResp)
	})
}
