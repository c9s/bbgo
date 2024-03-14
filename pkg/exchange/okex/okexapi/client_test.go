package okexapi

import (
	"context"
	"fmt"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"os"
	"strconv"
	"testing"
	"time"

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

func TestClient_OrderHistoryWithBeforeId(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	orders := []OrderDetail{}
	beforeId := int64(0)
	for {
		//>> [{"accFillSz":"0.00001","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"48174.5","cTime":"1704957916401","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.000385396","feeCcy":"USDT","fillPx":"48174.5","fillSz":"0.00001","fillTime":"1704983881118","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665576973905014786","ordType":"limit","pnl":"0","posSide":"","px":"48174.5","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.00001","tag":"","tdMode":"cash","tgtCcy":"","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"472610696","uTime":"1704983881135"}]
		//>> [{"accFillSz":"0.00001","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"48074.5","cTime":"1704957905283","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.000384596","feeCcy":"USDT","fillPx":"48074.5","fillSz":"0.00001","fillTime":"1704983824237","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665576927272742919","ordType":"limit","pnl":"0","posSide":"","px":"48074.5","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.00001","tag":"","tdMode":"cash","tgtCcy":"","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"472601591","uTime":"1704983824240"}]
		//>> [{"accFillSz":"0.00001","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"48073.5","cTime":"1704957892896","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.000384588","feeCcy":"USDT","fillPx":"48073.5","fillSz":"0.00001","fillTime":"1704983824227","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665576875317899302","ordType":"limit","pnl":"0","posSide":"","px":"48073.5","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.00001","tag":"","tdMode":"cash","tgtCcy":"","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"472601583","uTime":"1704983824230"}]
		//>> [{"accFillSz":"0.00016266","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"45919.8","cTime":"1704852215160","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.00000016266","feeCcy":"BTC","fillPx":"45919.8","fillSz":"0.00016266","fillTime":"1704852215162","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665133630767091729","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.00016266","tag":"","tdMode":"cash","tgtCcy":"base_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"471113058","uTime":"1704852215163"}]
		//>> [{"accFillSz":"0.00087627","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"45647.6","cTime":"1704850530651","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.00000087627","feeCcy":"BTC","fillPx":"45647.6","fillSz":"0.00087627","fillTime":"1704850530652","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665126565424254976","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"40","tag":"","tdMode":"cash","tgtCcy":"quote_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"471105716","uTime":"1704850530654"}]
		//>> [{"accFillSz":"0.001","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"45661.3","cTime":"1704850506060","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.0456613","feeCcy":"USDT","fillPx":"45661.3","fillSz":"0.001","fillTime":"1704850506061","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665126462282125313","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.001","tag":"","tdMode":"cash","tgtCcy":"base_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"471105593","uTime":"1704850506062"}]
		//>> [{"accFillSz":"0.00097361","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"45743","cTime":"1704849690516","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.00000097361","feeCcy":"BTC","fillPx":"45743","fillSz":"0.00097361","fillTime":"1704849690517","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665123041642663944","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.00097361","tag":"","tdMode":"cash","tgtCcy":"base_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"471100149","uTime":"1704849690519"}]
		//>> [{"accFillSz":"0.00080894","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"46728.2","cTime":"1704789666800","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.037800310108","feeCcy":"USDT","fillPx":"46728.2","fillSz":"0.00080894","fillTime":"1704789666801","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"664871283930550273","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"37.8","tag":"","tdMode":"cash","tgtCcy":"quote_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"470288552","uTime":"1704789666803"}]
		//>> [{"accFillSz":"0.00085423","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"46825.3","cTime":"1704789220044","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.00000085423","feeCcy":"BTC","fillPx":"46825.3","fillSz":"0.00085423","fillTime":"1704789220045","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"664869410100072448","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"40","tag":"","tdMode":"cash","tgtCcy":"quote_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"470287675","uTime":"1704789220046"}]
		c := client.NewGetOrderHistoryRequest().InstrumentID("BTC-USDT").Limit(1).Before(fmt.Sprintf("%d", beforeId))
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

func TestClient_OrderHistoryByTimeRange(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	startTime := time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC)
	t.Log(time.Since(startTime))
	//>> [{"accFillSz":"0.00001","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"48174.5","cTime":"1704957916401","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.000385396","feeCcy":"USDT","fillPx":"48174.5","fillSz":"0.00001","fillTime":"1704983881118","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665576973905014786","ordType":"limit","pnl":"0","posSide":"","px":"48174.5","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.00001","tag":"","tdMode":"cash","tgtCcy":"","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"472610696","uTime":"1704983881135"}]
	//>> [{"accFillSz":"0.00001","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"48074.5","cTime":"1704957905283","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.000384596","feeCcy":"USDT","fillPx":"48074.5","fillSz":"0.00001","fillTime":"1704983824237","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665576927272742919","ordType":"limit","pnl":"0","posSide":"","px":"48074.5","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.00001","tag":"","tdMode":"cash","tgtCcy":"","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"472601591","uTime":"1704983824240"}]
	//>> [{"accFillSz":"0.00001","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"48073.5","cTime":"1704957892896","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.000384588","feeCcy":"USDT","fillPx":"48073.5","fillSz":"0.00001","fillTime":"1704983824227","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665576875317899302","ordType":"limit","pnl":"0","posSide":"","px":"48073.5","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.00001","tag":"","tdMode":"cash","tgtCcy":"","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"472601583","uTime":"1704983824230"}]
	//>> [{"accFillSz":"0.00016266","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"45919.8","cTime":"1704852215160","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.00000016266","feeCcy":"BTC","fillPx":"45919.8","fillSz":"0.00016266","fillTime":"1704852215162","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665133630767091729","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.00016266","tag":"","tdMode":"cash","tgtCcy":"base_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"471113058","uTime":"1704852215163"}]
	//>> [{"accFillSz":"0.00087627","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"45647.6","cTime":"1704850530651","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.00000087627","feeCcy":"BTC","fillPx":"45647.6","fillSz":"0.00087627","fillTime":"1704850530652","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665126565424254976","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"40","tag":"","tdMode":"cash","tgtCcy":"quote_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"471105716","uTime":"1704850530654"}]
	//>> [{"accFillSz":"0.001","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"45661.3","cTime":"1704850506060","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.0456613","feeCcy":"USDT","fillPx":"45661.3","fillSz":"0.001","fillTime":"1704850506061","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665126462282125313","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.001","tag":"","tdMode":"cash","tgtCcy":"base_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"471105593","uTime":"1704850506062"}]
	//>> [{"accFillSz":"0.00097361","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"45743","cTime":"1704849690516","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.00000097361","feeCcy":"BTC","fillPx":"45743","fillSz":"0.00097361","fillTime":"1704849690517","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"665123041642663944","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"0.00097361","tag":"","tdMode":"cash","tgtCcy":"base_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"471100149","uTime":"1704849690519"}]
	//>> [{"accFillSz":"0.00080894","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"46728.2","cTime":"1704789666800","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.037800310108","feeCcy":"USDT","fillPx":"46728.2","fillSz":"0.00080894","fillTime":"1704789666801","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"664871283930550273","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"BTC","reduceOnly":"false","side":"sell","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"37.8","tag":"","tdMode":"cash","tgtCcy":"quote_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"470288552","uTime":"1704789666803"}]
	//>> [{"accFillSz":"0.00085423","algoClOrdId":"","algoId":"","attachAlgoClOrdId":"","attachAlgoOrds":[],"avgPx":"46825.3","cTime":"1704789220044","cancelSource":"","cancelSourceReason":"","category":"normal","ccy":"","clOrdId":"","fee":"-0.00000085423","feeCcy":"BTC","fillPx":"46825.3","fillSz":"0.00085423","fillTime":"1704789220045","instId":"BTC-USDT","instType":"SPOT","lever":"","ordId":"664869410100072448","ordType":"market","pnl":"0","posSide":"","px":"","pxType":"","pxUsd":"","pxVol":"","quickMgnType":"","rebate":"0","rebateCcy":"USDT","reduceOnly":"false","side":"buy","slOrdPx":"","slTriggerPx":"","slTriggerPxType":"","source":"","state":"filled","stpId":"","stpMode":"","sz":"40","tag":"","tdMode":"cash","tgtCcy":"quote_ccy","tpOrdPx":"","tpTriggerPx":"","tpTriggerPxType":"","tradeId":"470287675","uTime":"1704789220046"}]
	c := client.NewGetOrderHistoryRequest().InstrumentID("BTC-USDT").Limit(100).After("665576927272742919").StartTime(types.NewMillisecondTimestampFromInt(1704789220044).Time())
	res, err := c.Do(ctx)
	assert.NoError(t, err)
	t.Log(res)
}

func TestClient_TransactionHistoryByOrderId(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	c := client.NewGetTransactionHistoryRequest().OrderID("665951812901531754")
	res, err := c.Do(ctx)
	assert.NoError(t, err)
	t.Log(res)
}

func TestClient_TransactionHistoryAll(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	beforeId := int64(0)
	for {
		c := client.NewGetTransactionHistoryRequest().Before(strconv.FormatInt(beforeId, 10)).Limit(1)
		res, err := c.Do(ctx)
		assert.NoError(t, err)
		t.Log(res)

		if len(res) != 1 {
			break
		}
		//orders = append(orders, res...)
		beforeId = int64(res[0].BillId)
		t.Log(res[0])
	}
}

func TestClient_TransactionHistoryWithTime(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	beforeId := int64(0)
	for {
		// [{"side":"sell","fillSz":"1","fillPx":"46446.4","fillPxVol":"","fillFwdPx":"","fee":"-46.4464","fillPnl":"0","ordId":"665951654130348158","feeRate":"-0.001","instType":"SPOT","fillPxUsd":"","instId":"BTC-USDT","clOrdId":"","posSide":"net","billId":"665951654138736652","fillMarkVol":"","tag":"","fillTime":"1705047247128","execType":"T","fillIdxPx":"","tradeId":"724072849","fillMarkPx":"","feeCcy":"USDT","ts":"1705047247130"}]
		// [{"side":"sell","fillSz":"11.053006","fillPx":"54.17","fillPxVol":"","fillFwdPx":"","fee":"-0.59874133502","fillPnl":"0","ordId":"665951812901531754","feeRate":"-0.001","instType":"SPOT","fillPxUsd":"","instId":"OKB-USDT","clOrdId":"","posSide":"net","billId":"665951812905726068","fillMarkVol":"","tag":"","fillTime":"1705047284982","execType":"T","fillIdxPx":"","tradeId":"589438381","fillMarkPx":"","feeCcy":"USDT","ts":"1705047284983"}]
		// [{"side":"sell","fillSz":"88.946994","fillPx":"54.16","fillPxVol":"","fillFwdPx":"","fee":"-4.81736919504","fillPnl":"0","ordId":"665951812901531754","feeRate":"-0.001","instType":"SPOT","fillPxUsd":"","instId":"OKB-USDT","clOrdId":"","posSide":"net","billId":"665951812905726084","fillMarkVol":"","tag":"","fillTime":"1705047284982","execType":"T","fillIdxPx":"","tradeId":"589438382","fillMarkPx":"","feeCcy":"USDT","ts":"1705047284983"}]
		c := client.NewGetTransactionHistoryRequest().Limit(1).Before(fmt.Sprintf("%d", beforeId)).
			StartTime(types.NewMillisecondTimestampFromInt(1705047247130).Time()).
			EndTime(types.NewMillisecondTimestampFromInt(1705047284983).Time())
		res, err := c.Do(ctx)
		assert.NoError(t, err)
		t.Log(res)

		if len(res) != 1 {
			break
		}
		beforeId = int64(res[0].BillId)
	}
}

func TestClient_ThreeDaysTransactionHistoryWithTime(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	beforeId := int64(0)
	startTime := time.Now().Add(-3 * 24 * time.Hour)
	end := time.Now()

	for {
		// [{"side":"sell","fillSz":"1","fillPx":"46446.4","fillPxVol":"","fillFwdPx":"","fee":"-46.4464","fillPnl":"0","ordId":"665951654130348158","feeRate":"-0.001","instType":"SPOT","fillPxUsd":"","instId":"BTC-USDT","clOrdId":"","posSide":"net","billId":"665951654138736652","fillMarkVol":"","tag":"","fillTime":"1705047247128","execType":"T","fillIdxPx":"","tradeId":"724072849","fillMarkPx":"","feeCcy":"USDT","ts":"1705047247130"}]
		// [{"side":"sell","fillSz":"11.053006","fillPx":"54.17","fillPxVol":"","fillFwdPx":"","fee":"-0.59874133502","fillPnl":"0","ordId":"665951812901531754","feeRate":"-0.001","instType":"SPOT","fillPxUsd":"","instId":"OKB-USDT","clOrdId":"","posSide":"net","billId":"665951812905726068","fillMarkVol":"","tag":"","fillTime":"1705047284982","execType":"T","fillIdxPx":"","tradeId":"589438381","fillMarkPx":"","feeCcy":"USDT","ts":"1705047284983"}]
		// [{"side":"sell","fillSz":"88.946994","filollPx":"54.16","fillPxVol":"","fillFwdPx":"","fee":"-4.81736919504","fillPnl":"0","ordId":"665951812901531754","feeRate":"-0.001","instType":"SPOT","fillPxUsd":"","instId":"OKB-USDT","clOrdId":"","posSide":"net","billId":"665951812905726084","fillMarkVol":"","tag":"","fillTime":"1705047284982","execType":"T","fillIdxPx":"","tradeId":"589438382","fillMarkPx":"","feeCcy":"USDT","ts":"1705047284983"}]
		c := client.NewGetThreeDaysTransactionHistoryRequest().
			StartTime(types.NewMillisecondTimestampFromInt(startTime.UnixMilli()).Time()).
			EndTime(types.NewMillisecondTimestampFromInt(end.UnixMilli()).Time()).
			Limit(1)
		if beforeId != 0 {
			c.Before(strconv.FormatInt(beforeId, 10))
		}
		res, err := c.Do(ctx)
		assert.NoError(t, err)

		if len(res) != 1 {
			break
		}
		t.Log(res[0].FillTime, res[0].Timestamp, res[0].BillId, res)
		beforeId = int64(res[0].BillId)
	}
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

func TestClient_CandlesTicksRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()
	req := client.NewGetCandlesRequest().InstrumentID("BTC-USDT")
	res, err := req.Do(ctx)
	assert.NoError(t, err)
	t.Log(res)
}
