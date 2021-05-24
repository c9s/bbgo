package ftx

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func Test_messageHandler_handleMessage(t *testing.T) {
	t.Run("handle order update", func(t *testing.T) {
		input := []byte(`
{
  "channel": "orders",
  "type": "update",
  "data": {
    "id": 36379,
    "clientId": null,
    "market": "OXY-PERP",
    "type": "limit",
    "side": "sell",
    "price": 2.7185,
    "size": 1.0,
    "status": "closed",
    "filledSize": 1.0,
    "remainingSize": 0.0,
    "reduceOnly": false,
    "liquidation": false,
    "avgFillPrice": 2.7185,
    "postOnly": false,
    "ioc": false,
    "createdAt": "2021-03-28T06:12:50.991447+00:00"
  }
}
`)

		h := &messageHandler{StandardStream: &types.StandardStream{}}
		i := 0
		h.OnOrderUpdate(func(order types.Order) {
			i++
			assert.Equal(t, types.Order{
				SubmitOrder: types.SubmitOrder{
					ClientOrderID: "",
					Symbol:        "OXY-PERP",
					Side:          types.SideTypeSell,
					Type:          types.OrderTypeLimit,
					Quantity:      1.0,
					Price:         2.7185,
					TimeInForce:   "GTC",
				},
				Exchange:         types.ExchangeFTX,
				OrderID:          36379,
				Status:           types.OrderStatusFilled,
				ExecutedQuantity: 1.0,
				CreationTime:     types.Time(mustParseDatetime("2021-03-28T06:12:50.991447+00:00")),
				UpdateTime:       types.Time(mustParseDatetime("2021-03-28T06:12:50.991447+00:00")),
			}, order)
		})
		h.handleMessage(input)
		assert.Equal(t, 1, i)
	})

	t.Run("handle trade update", func(t *testing.T) {
		input := []byte(`
{
  "channel": "fills",
  "type": "update",
  "data": {
    "id": 23427,
    "market": "OXY-PERP",
    "future": "OXY-PERP",
    "baseCurrency": null,
    "quoteCurrency": null,
    "type": "order",
    "side": "buy",
    "price": 2.723,
    "size": 1.0,
    "orderId": 323789,
    "time": "2021-03-28T06:12:34.702926+00:00",
    "tradeId": 6276431,
    "feeRate": 0.00056525,
    "fee": 0.00153917575,
    "feeCurrency": "USD",
    "liquidity": "taker"
  }
}
`)
		h := &messageHandler{StandardStream: &types.StandardStream{}}
		i := 0
		h.OnTradeUpdate(func(trade types.Trade) {
			i++
			assert.Equal(t, types.Trade{
				GID:           0,
				ID:            6276431,
				OrderID:       323789,
				Exchange:      types.ExchangeFTX,
				Price:         2.723,
				Quantity:      1.0,
				QuoteQuantity: 2.723 * 1.0,
				Symbol:        "OXY-PERP",
				Side:          types.SideTypeBuy,
				IsBuyer:       true,
				IsMaker:       false,
				Time:          types.Time(mustParseDatetime("2021-03-28T06:12:34.702926+00:00")),
				Fee:           0.00153917575,
				FeeCurrency:   "USD",
				IsMargin:      false,
				IsIsolated:    false,
				StrategyID:    sql.NullString{},
				PnL:           sql.NullFloat64{},
			}, trade)
		})
		h.handleMessage(input)
		assert.Equal(t, 1, i)
	})
}
