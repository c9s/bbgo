package okexapi

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
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
	req := client.NewGetInstrumentsInfoRequest()

	instruments, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, instruments)
	t.Logf("instruments: %+v", instruments)
}

func TestClient_GetMarketTickers(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	req := client.NewGetTickersRequest()

	tickers, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, tickers)
	t.Logf("tickers: %+v", tickers)
}

func TestClient_GetMarketTicker(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	req := client.NewGetTickerRequest().InstId("BTC-USDT")

	tickers, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, tickers)
	t.Logf("tickers: %+v", tickers)
}

func TestClient_GetAcountInfo(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	req := client.NewGetAccountInfoRequest()

	acct, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, acct)
	t.Logf("acct: %+v", acct)
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
		TradeMode(TradeModeCash).
		Side(SideTypeSell).
		OrderType(OrderTypeLimit).
		TargetCurrency(TargetCurrencyBase).
		Price("48000").
		Size("0.001").
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, order)
	t.Logf("place order: %+v", order)

	c := client.NewGetOrderDetailsRequest().OrderID(order[0].OrderID).InstrumentID("BTC-USDT")
	res, err := c.Do(ctx)
	assert.NoError(t, err)
	t.Log(res)
}

func TestClient_CancelOrderRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	req := client.NewPlaceOrderRequest()
	clientId := fmt.Sprintf("%d", uuid.New().ID())

	order, err := req.
		InstrumentID("BTC-USDT").
		TradeMode(TradeModeCash).
		Side(SideTypeSell).
		OrderType(OrderTypeLimit).
		TargetCurrency(TargetCurrencyBase).
		ClientOrderID(clientId).
		Price("48000").
		Size("0.001").
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, order)
	t.Logf("place order: %+v", order)

	c := client.NewGetOrderDetailsRequest().ClientOrderID(clientId).InstrumentID("BTC-USDT")
	res, err := c.Do(ctx)
	assert.NoError(t, err)
	t.Log(res)

	cancelResp, err := client.NewCancelOrderRequest().ClientOrderID(clientId).InstrumentID("BTC-USDT").Do(ctx)
	assert.NoError(t, err)
	t.Log(cancelResp)
}

func TestClient_OpenOrdersRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	orders := []OpenOrder{}
	beforeId := int64(0)
	for {
		c := client.NewGetOpenOrdersRequest().InstrumentID("BTC-USDT").Limit("1").After(fmt.Sprintf("%d", beforeId))
		res, err := c.Do(ctx)
		assert.NoError(t, err)
		if len(res) != 1 {
			break
		}
		orders = append(orders, res...)
		beforeId = int64(res[0].OrderId)
	}

	t.Log(orders)
}

func TestClient_BatchCancelOrderRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	req := client.NewPlaceOrderRequest()
	clientId := fmt.Sprintf("%d", uuid.New().ID())

	order, err := req.
		InstrumentID("BTC-USDT").
		TradeMode(TradeModeCash).
		Side(SideTypeSell).
		OrderType(OrderTypeLimit).
		TargetCurrency(TargetCurrencyBase).
		ClientOrderID(clientId).
		Price("48000").
		Size("0.001").
		Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, order)
	t.Logf("place order: %+v", order)

	c := client.NewGetOrderDetailsRequest().ClientOrderID(clientId).InstrumentID("BTC-USDT")
	res, err := c.Do(ctx)
	assert.NoError(t, err)
	t.Log(res)

	cancelResp, err := client.NewBatchCancelOrderRequest().Add(&CancelOrderRequest{instrumentID: "BTC-USDT", clientOrderID: &clientId}).Do(ctx)
	assert.NoError(t, err)
	t.Log(cancelResp)
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
