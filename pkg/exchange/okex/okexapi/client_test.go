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

	client, err := NewClient()
	assert.NoError(t, err)
	client.Auth(key, secret, passphrase)
	return client
}

func TestClient_GetInstrumentsRequest(t *testing.T) {
	client, err := NewClient()
	assert.NoError(t, err)
	ctx := context.Background()

	// srv := &PublicDataService{client: client}
	req := client.NewGetInstrumentsRequest()

	instruments, err := req.
		InstrumentType(InstrumentTypeSpot).
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, instruments)
	t.Logf("instruments: %+v", instruments)
}

func TestClient_GetFundingRateRequest(t *testing.T) {
	client, err := NewClient()
	assert.NoError(t, err)
	ctx := context.Background()
	// srv := &PublicDataService{client: client}
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
	// srv := &TradeService{client: client}
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
	// srv := &TradeService{client: client}
	req := client.NewGetOrderDetailsRequest()

	orderDetail, err := req.
		InstrumentID("BTC-USDT").
		OrderID("609869603774656544").
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, orderDetail)
	t.Logf("order detail: %+v", orderDetail)
}

func TestClient_GetTransactionDetailsRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	// srv := &TradeService{client: client}
	req := client.NewGetTransactionDetailsRequest()

	transactionDetail, err := req.
		InstrumentType(InstrumentTypeSpot).
		Do(ctx)

	assert.NoError(t, err)
	assert.Empty(t, transactionDetail) // No transaction in 3 days
	t.Logf("transaction detail: %+v", transactionDetail)
}

func TestClient_GetAccountBalanceRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	account, err := client.AccountBalances(ctx)

	assert.NoError(t, err)
	assert.NotEmpty(t, account)
	t.Logf("account detail: %+v", account)
}

func TestClient_GetAssetBalancesRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	asset, err := client.AssetBalances(ctx)

	assert.NoError(t, err)
	assert.Empty(t, asset)
	t.Logf("asset detail: %+v", asset)
}

func TestClient_GetAssetCurrenciesRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	asset, err := client.AssetCurrencies(ctx)

	assert.NoError(t, err)
	assert.NotEmpty(t, asset)
	t.Logf("asset detail: %+v", asset)
}

func TestClient_GetMarketTickerRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	ticker, err := client.MarketTicker(ctx, "BTC-USDT")

	assert.NoError(t, err)
	assert.NotEmpty(t, ticker)
	t.Logf("ticker detail: %+v", ticker)
}

func TestClient_GetMarketTickersRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	ticker, err := client.MarketTickers(ctx, "SPOT")

	assert.NoError(t, err)
	assert.NotEmpty(t, ticker)
	t.Logf("ticker detail: %+v", ticker)
}
