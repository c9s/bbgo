package okexapi

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func getTestClientOrSkip(t *testing.T) *RestClient {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.Skip("Please configure all credentials about OKEX")
		return nil
	}

	client := NewClient()
	client.Auth(key, secret, passphrase)
	return client
}

func TestClient_GetInstrumentsRequest(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	req := client.NewGetInstrumentsRequest()

	instruments, err := req.
		InstrumentType(InstrumentTypeSpot).
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, instruments)
	t.Logf("instruments: %+v", instruments)
}

func TestClient_GetFundingRateRequest(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	req := client.NewGetFundingRate()

	instrument, err := req.
		InstrumentID("BTC-USDT-SWAP").
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, instrument)
	t.Logf("instrument: %+v", instrument)
}

func TestClient_PlaceOrderRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	req := client.NewPlaceOrderRequest()

	order, err := req.
		InstrumentID("BTC-USDT").
		TradeMode("cash").
		Side(SideTypeBuy).
		OrderType(OrderTypeLimit).
		Price("15000").
		Quantity("0.0001").
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, order)
	t.Logf("place order: %+v", order)
}

func TestClient_GetPendingOrderRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	req := client.NewGetPendingOrderRequest()
	odr_type := []string{string(OrderTypeLimit), string(OrderTypeIOC)}

	pending_order, err := req.
		InstrumentID("BTC-USDT").
		OrderTypes(odr_type).
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, pending_order)
	t.Logf("pending order: %+v", pending_order)
}

func TestClient_GetOrderDetailsRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	req := client.NewGetOrderDetailsRequest()

	orderDetail, err := req.
		InstrumentID("BTC-USDT").
		OrderID("609869603774656544").
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, orderDetail)
	t.Logf("order detail: %+v", orderDetail)
}
