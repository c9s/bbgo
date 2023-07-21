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
		t.SkipNow()
		return nil
	}

	client := NewClient()
	client.Auth(key, secret, passphrase)
	return client
}

func TestClient_GetInstrumentsRequest(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	ser := PublicDataService{client: client}
	req := ser.NewGetInstrumentsRequest()

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
	ser := PublicDataService{client: client}
	req := ser.NewGetFundingRate()

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
	ser := TradeService{client: client}
	req := ser.NewPlaceOrderRequest()

	order, err := req.
		InstrumentID("XTZ-BTC").
		TradeMode("cash").
		Side(SideTypeSell).
		OrderType(OrderTypeLimit).
		Price("0.001").
		Quantity("0.01").
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, order)
	t.Logf("order: %+v", order) // Right now account has no money
}
