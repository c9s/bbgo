package binanceapi

import (
	"context"
	"log"
	"net/http/httputil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func getTestClientOrSkip(t *testing.T) *RestClient {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BINANCE")
	if !ok {
		t.SkipNow()
		return nil
	}

	client := NewClient("")
	client.Auth(key, secret)
	return client
}

func TestClient_GetTradeFeeRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req := client.NewGetTradeFeeRequest()
	tradeFees, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, tradeFees)
	t.Logf("tradeFees: %+v", tradeFees)
}

func TestClient_GetDepositAddressRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req := client.NewGetDepositAddressRequest()
	req.Coin("BTC")
	address, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, address)
	assert.NotEmpty(t, address.Url)
	assert.NotEmpty(t, address.Address)
	t.Logf("deposit address: %+v", address)
}

func TestClient_GetDepositHistoryRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req := client.NewGetDepositHistoryRequest()
	history, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, history)
	assert.NotEmpty(t, history)
	t.Logf("deposit history: %+v", history)
}

func TestClient_NewSpotRebateHistoryRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req := client.NewGetSpotRebateHistoryRequest()
	history, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, history)
	assert.NotEmpty(t, history)
	t.Logf("spot rebate history: %+v", history)
}

func TestClient_NewGetMarginInterestRateHistoryRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req := client.NewGetMarginInterestRateHistoryRequest()
	req.Asset("BTC")
	history, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, history)
	assert.NotEmpty(t, history)
	t.Logf("interest rate history: %+v", history)
}

func TestClient_privateCall(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BINANCE")
	if !ok {
		t.SkipNow()
	}

	client := NewClient("")
	client.Auth(key, secret)

	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req, err := client.NewAuthenticatedRequest(ctx, "GET", "/sapi/v1/asset/tradeFee", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, req)

	resp, err := client.SendRequest(req)
	if assert.NoError(t, err) {
		var feeStructs []struct {
			Symbol          string `json:"symbol"`
			MakerCommission string `json:"makerCommission"`
			TakerCommission string `json:"takerCommission"`
		}
		err = resp.DecodeJSON(&feeStructs)
		if assert.NoError(t, err) {
			assert.NotEmpty(t, feeStructs)
		}
	} else {
		dump, _ := httputil.DumpRequest(req, true)
		log.Printf("request: %s", dump)
	}
}

func TestClient_setTimeOffsetFromServer(t *testing.T) {
	client := NewClient("")
	err := client.SetTimeOffsetFromServer(context.Background())
	assert.NoError(t, err)
}
