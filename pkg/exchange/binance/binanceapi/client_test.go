package binanceapi

import (
	"context"
	"log"
	"net/http/httputil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testutil"
)

func getTestClientOrSkip(t *testing.T) *RestClient {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

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
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

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
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	client := NewClient("")
	err := client.SetTimeOffsetFromServer(context.Background())
	assert.NoError(t, err)
}

func TestClient_NewTransferAssetRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req := client.NewTransferAssetRequest()
	req.Asset("BTC")
	req.FromSymbol("BTCUSDT")
	req.ToSymbol("BTCUSDT")
	req.Amount("0.01")
	req.TransferType(TransferAssetTypeIsolatedMarginToMain)
	res, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotEmpty(t, res)
	t.Logf("result: %+v", res)
}

func TestClient_GetMarginBorrowRepayHistoryRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req := client.NewGetMarginBorrowRepayHistoryRequest()
	end := time.Now()
	start := end.Add(-24 * time.Hour * 30)
	req.StartTime(start)
	req.EndTime(end)
	req.Asset("BTC")
	req.SetBorrowRepayType(BorrowRepayTypeBorrow)
	res, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotEmpty(t, res)
	t.Logf("result: %+v", res)
}

func TestClient_NewPlaceMarginOrderRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	res, err := client.NewPlaceMarginOrderRequest().
		Asset("USDT").
		Amount(fixedpoint.NewFromFloat(5)).
		IsIsolated(true).
		Symbol("BNBUSDT").
		SetBorrowRepayType(BorrowRepayTypeBorrow).
		Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotEmpty(t, res)
	t.Logf("result: %+v", res)

	<-time.After(time.Second)
	end := time.Now()
	start := end.Add(-24 * time.Hour * 30)
	histories, err := client.NewGetMarginBorrowRepayHistoryRequest().
		StartTime(start).
		EndTime(end).
		Asset("BNB").
		IsolatedSymbol("BNBUSDT").
		SetBorrowRepayType(BorrowRepayTypeBorrow).
		Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, histories)
	assert.NotEmpty(t, histories)
	t.Logf("result: %+v", histories)

	res, err = client.NewPlaceMarginOrderRequest().
		Asset("USDT").
		Amount(fixedpoint.NewFromFloat(5)).
		IsIsolated(true).
		Symbol("BNBUSDT").
		SetBorrowRepayType(BorrowRepayTypeRepay).
		Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotEmpty(t, res)
	t.Logf("result: %+v", res)
}

func TestClient_GetDepth(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req := client.NewGetDepthRequest().Symbol("BTCUSDT").Limit(1000)
	resp, err := req.Do(ctx)
	if assert.NoError(t, err) {
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp)
		t.Logf("response: %+v", resp)
	}
}
