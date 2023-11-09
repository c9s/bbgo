package bitgetapi

import (
	"context"
	"os"
	"strconv"
	"testing"

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
		req := client.NewGetUnfilledOrdersRequest().StartTime(1)
		resp, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("resp: %+v", resp)
	})

	t.Run("GetHistoryOrdersRequest", func(t *testing.T) {
		// market buy
		req, err := client.NewGetHistoryOrdersRequest().Symbol("APEUSDT").Do(ctx)
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
		req, err := client.NewGetTradeFillsRequest().Symbol("APEUSDT").Do(ctx)
		assert.NoError(t, err)

		t.Logf("get trade fills resp: %+v", req)
	})
}
