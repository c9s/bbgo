package bitgetapi

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	"github.com/c9s/bbgo/pkg/testutil"
)

func getTestClientOrSkip(t *testing.T) *Client {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BITGET")
	if !ok {
		t.Skip("BITGET_* env vars are not configured")
		return nil
	}

	client := bitgetapi.NewClient()
	client.Auth(key, secret, os.Getenv("BITGET_API_PASSPHRASE"))
	return NewClient(client)
}

func TestClient(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	t.Run("GetUnfilledOrdersRequest", func(t *testing.T) {
		startTime := time.Now().Add(-30 * 24 * time.Hour)
		req := client.NewGetUnfilledOrdersRequest().StartTime(startTime)
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("resp: %+v", resp)
	})

	t.Run("GetHistoryOrdersRequest", func(t *testing.T) {
		startTime := time.Now().Add(-30 * 24 * time.Hour)
		req, err := client.NewGetHistoryOrdersRequest().Symbol("APEUSDT").StartTime(startTime).Do(ctx)
		assert.NoError(t, err)

		t.Logf("place order resp: %+v", req)
	})

	t.Run("PlaceOrderRequest", func(t *testing.T) {
		req, err := client.NewPlaceOrderRequest().Symbol("APEUSDT").OrderType(OrderTypeLimit).
			Side(SideTypeSell).
			Price("2").
			Size("5").
			Force(OrderForceGTC).
			Do(context.Background())
		assert.NoError(t, err)

		t.Logf("place order resp: %+v", req)
	})

	t.Run("GetTradeFillsRequest", func(t *testing.T) {
		startTime := time.Now().Add(-30 * 24 * time.Hour)
		req, err := client.NewGetTradeFillsRequest().Symbol("APEUSDT").StartTime(startTime).Do(ctx)
		assert.NoError(t, err)

		t.Logf("get trade fills resp: %+v", req)
	})

	t.Run("CancelOrderRequest", func(t *testing.T) {
		req, err := client.NewPlaceOrderRequest().Symbol("APEUSDT").OrderType(OrderTypeLimit).
			Side(SideTypeSell).
			Price("2").
			Size("5").
			Force(OrderForceGTC).
			Do(context.Background())
		assert.NoError(t, err)

		resp, err := client.NewCancelOrderRequest().Symbol("APEUSDT").OrderId(req.OrderId).Do(ctx)
		t.Logf("cancel order resp: %+v", resp)
	})

	t.Run("GetKLineRequest", func(t *testing.T) {
		startTime := time.Date(2023, 8, 12, 0, 0, 0, 0, time.UTC)
		endTime := time.Date(2023, 10, 14, 0, 0, 0, 0, time.UTC)
		resp, err := client.NewGetKLineRequest().Symbol("APEUSDT").Granularity("30min").StartTime(startTime).EndTime(endTime).Limit("1000").Do(ctx)
		assert.NoError(t, err)
		t.Logf("resp: %+v", resp)
	})

	t.Run("GetSymbolsRequest", func(t *testing.T) {
		resp, err := client.NewGetSymbolsRequest().Do(ctx)
		assert.NoError(t, err)
		t.Logf("resp: %+v", resp)
	})

	t.Run("GetTickersRequest", func(t *testing.T) {
		resp, err := client.NewGetTickersRequest().Do(ctx)
		assert.NoError(t, err)
		t.Logf("resp: %+v", resp)
	})

	t.Run("GetAccountAssetsRequest", func(t *testing.T) {
		resp, err := client.NewGetAccountAssetsRequest().AssetType(AssetTypeHoldOnly).Do(ctx)
		assert.NoError(t, err)
		t.Logf("resp: %+v", resp)
	})
}
