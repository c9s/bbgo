package binance

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestCancelReplaceSpotReturnsReplacementAndSendsNewClientOrderID(t *testing.T) {
	transport := &httptesting.MockTransport{}
	ex := &Exchange{client2: binanceapi.NewClient("https://api.binance.test")}
	ex.client2.Auth("test-api-key", "test-api-secret", nil)
	ex.client2.HttpClient = &http.Client{Transport: transport}

	var requestPayload string
	transport.POST("/api/v3/order/cancelReplace", func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		requestPayload = req.URL.RawQuery + "&" + string(body)
		return httptesting.BuildResponseString(http.StatusOK, `{
			"data": {
				"cancelResult": "SUCCESS",
				"newOrderResult": "SUCCESS",
				"newOrderResponse": {
					"symbol": "ETHJPY",
					"orderId": 456,
					"clientOrderId": "maker-replacement-1",
					"price": "300000",
					"origQty": "0.001",
					"executedQty": "0",
					"cummulativeQuoteQty": "0",
					"status": "NEW",
					"timeInForce": "GTC",
					"type": "LIMIT_MAKER",
					"side": "BUY",
					"isWorking": true
				}
			}
		}`), nil
	})

	order := types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: "maker-replacement-1",
			Symbol:        "ETHJPY",
			Side:          types.SideTypeBuy,
			Type:          types.OrderTypeLimitMaker,
			Quantity:      fixedpoint.MustNewFromString("0.001"),
			Price:         fixedpoint.MustNewFromString("300000"),
		},
		OrderID: 123,
	}

	got, err := ex.CancelReplace(context.Background(), types.StopOnFailure, order)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, uint64(456), got.OrderID)
	require.Equal(t, "maker-replacement-1", got.ClientOrderID)
	require.True(t, strings.Contains(requestPayload, "newClientOrderId=maker-replacement-1"), requestPayload)
}

func TestCancelReplaceSpotReturnsErrorOnPartialResult(t *testing.T) {
	transport := &httptesting.MockTransport{}
	ex := &Exchange{client2: binanceapi.NewClient("https://api.binance.test")}
	ex.client2.Auth("test-api-key", "test-api-secret", nil)
	ex.client2.HttpClient = &http.Client{Transport: transport}
	transport.POST("/api/v3/order/cancelReplace", func(_ *http.Request) (*http.Response, error) {
		return httptesting.BuildResponseString(http.StatusOK, `{
			"data": {
				"cancelResult": "SUCCESS",
				"newOrderResult": "FAILURE",
				"newOrderResponse": null
			}
		}`), nil
	})

	order := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   "ETHJPY",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimitMaker,
			Quantity: fixedpoint.MustNewFromString("0.001"),
			Price:    fixedpoint.MustNewFromString("300000"),
		},
		OrderID: 123,
	}

	got, err := ex.CancelReplace(context.Background(), types.StopOnFailure, order)
	require.Error(t, err)
	require.Nil(t, got)
	require.Contains(t, err.Error(), "newOrderResult=FAILURE")
}
