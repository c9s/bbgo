package kucoin

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/types"
)

func TestExchange_SubmitOrder_ReturnsAPIOrder(t *testing.T) {
	const orderID = "672cf15bb2cdb8000708765c"
	const createdAt = int64(1547026471000)

	market := types.Market{
		Symbol:          "BTCUSDT",
		LocalSymbol:     "BTC-USDT",
		BaseCurrency:    "BTC",
		QuoteCurrency:   "USDT",
		PricePrecision:  2,
		VolumePrecision: 8,
		TickSize:        fixedpoint.MustNewFromString("0.01"),
		StepSize:        fixedpoint.MustNewFromString("0.00000001"),
	}

	cases := []struct {
		name             string
		submitOrder      types.SubmitOrder
		orderResponse    string
		expectedStatus   types.OrderStatus
		expectedExecuted fixedpoint.Value
		expectedWorking  bool
		expectedPrice    fixedpoint.Value
	}{
		{
			name: "filled market order",
			submitOrder: types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Market:   market,
				Side:     types.SideTypeBuy,
				Type:     types.OrderTypeMarket,
				Quantity: fixedpoint.NewFromFloat(2),
			},
			orderResponse:    `{"code":"200000","data":{"id":"` + orderID + `","symbol":"BTC-USDT","opType":"DEAL","type":"market","side":"buy","price":"101.5","size":"2","dealSize":"2","isActive":false,"cancelExist":false,"createdAt":1547026471000}}`,
			expectedStatus:   types.OrderStatusFilled,
			expectedExecuted: fixedpoint.NewFromFloat(2),
			expectedWorking:  false,
			expectedPrice:    fixedpoint.NewFromFloat(101.5),
		},
		{
			name: "partially filled limit order",
			submitOrder: types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Market:   market,
				Side:     types.SideTypeBuy,
				Type:     types.OrderTypeLimit,
				Price:    fixedpoint.NewFromFloat(100),
				Quantity: fixedpoint.NewFromFloat(2),
			},
			orderResponse:    `{"code":"200000","data":{"id":"` + orderID + `","symbol":"BTC-USDT","opType":"DEAL","type":"limit","side":"buy","price":"100","size":"2","dealSize":"0.5","isActive":true,"cancelExist":false,"createdAt":1547026471000}}`,
			expectedStatus:   types.OrderStatusPartiallyFilled,
			expectedExecuted: fixedpoint.NewFromFloat(0.5),
			expectedWorking:  true,
			expectedPrice:    fixedpoint.NewFromFloat(100),
		},
		{
			name: "new limit order",
			submitOrder: types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Market:   market,
				Side:     types.SideTypeBuy,
				Type:     types.OrderTypeLimit,
				Price:    fixedpoint.NewFromFloat(100),
				Quantity: fixedpoint.NewFromFloat(2),
			},
			orderResponse:    `{"code":"200000","data":{"id":"` + orderID + `","symbol":"BTC-USDT","opType":"DEAL","type":"limit","side":"buy","price":"100","size":"2","dealSize":"0","isActive":true,"cancelExist":false,"createdAt":1547026471000}}`,
			expectedStatus:   types.OrderStatusNew,
			expectedExecuted: fixedpoint.Zero,
			expectedWorking:  true,
			expectedPrice:    fixedpoint.NewFromFloat(100),
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			ex := New("key", "secret", "passphrase")

			transport := &httptesting.MockTransport{}
			transport.POST("/api/v1/orders", func(req *http.Request) (*http.Response, error) {
				return httptesting.BuildResponseString(http.StatusOK,
					`{"code":"200000","data":{"orderId":"`+orderID+`"}}`), nil
			})
			transport.GET("/api/v1/orders/"+orderID, func(req *http.Request) (*http.Response, error) {
				return httptesting.BuildResponseString(http.StatusOK, c.orderResponse), nil
			})
			ex.client.HttpClient.Transport = transport

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			createdOrder, err := ex.SubmitOrder(ctx, c.submitOrder)
			require.NoError(t, err)
			require.NotNil(t, createdOrder)

			assert.Equal(t, orderID, createdOrder.UUID)
			assert.Equal(t, hashStringID(orderID), createdOrder.OrderID)
			assert.Equal(t, c.expectedStatus, createdOrder.Status)
			assert.Equal(t, 0, c.expectedExecuted.Compare(createdOrder.ExecutedQuantity), "executed quantity")
			assert.Equal(t, c.expectedWorking, createdOrder.IsWorking)
			assert.Equal(t, 0, c.expectedPrice.Compare(createdOrder.Price), "price")
			assert.Equal(t, "BTCUSDT", createdOrder.Symbol)
			assert.Equal(t, c.submitOrder.Side, createdOrder.Side)
			assert.Equal(t, c.submitOrder.Type, createdOrder.Type)
			assert.Equal(t, time.UnixMilli(createdAt).UTC(), createdOrder.CreationTime.Time().UTC())
			assert.Equal(t, market, createdOrder.Market)
		})
	}
}
