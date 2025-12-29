package binance

import (
	"testing"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestToGlobalPositionRisk(t *testing.T) {
	positions := []binanceapi.FuturesPositionRisk{
		{
			Symbol:                 "BTCUSDT",
			PositionSide:           "LONG",
			MarkPrice:              fixedpoint.MustNewFromString("51000"),
			EntryPrice:             fixedpoint.MustNewFromString("50000"),
			PositionAmount:         fixedpoint.MustNewFromString("1.0"),
			BreakEvenPrice:         fixedpoint.MustNewFromString("49000"),
			UnRealizedProfit:       fixedpoint.MustNewFromString("1000"),
			InitialMargin:          fixedpoint.MustNewFromString("0.1"),
			Notional:               fixedpoint.MustNewFromString("51000"),
			PositionInitialMargin:  fixedpoint.MustNewFromString("0.1"),
			MaintMargin:            fixedpoint.MustNewFromString("0.05"),
			Adl:                    fixedpoint.MustNewFromString("1"),
			OpenOrderInitialMargin: fixedpoint.MustNewFromString("0.1"),
			UpdateTime:             types.MillisecondTimestamp(time.Unix(1234567890/1000, 0)),
			MarginAsset:            "USDT",
		},
	}

	risks := toGlobalPositionRisk(positions)
	assert.NotNil(t, risks)
	assert.Len(t, risks, 1)

	risk := risks[0]
	assert.Equal(t, "BTCUSDT", risk.Symbol)
	assert.Equal(t, types.PositionLong, risk.PositionSide)
	assert.Equal(t, fixedpoint.MustNewFromString("51000"), risk.MarkPrice)
	assert.Equal(t, fixedpoint.MustNewFromString("50000"), risk.EntryPrice)
	assert.Equal(t, fixedpoint.MustNewFromString("1.0"), risk.PositionAmount)
	assert.Equal(t, fixedpoint.MustNewFromString("49000"), risk.BreakEvenPrice)
	assert.Equal(t, fixedpoint.MustNewFromString("1000"), risk.UnrealizedPnL)
	assert.Equal(t, fixedpoint.MustNewFromString("0.1"), risk.InitialMargin)
	assert.Equal(t, fixedpoint.MustNewFromString("51000"), risk.Notional)
	assert.Equal(t, fixedpoint.MustNewFromString("0.1"), risk.PositionInitialMargin)
	assert.Equal(t, fixedpoint.MustNewFromString("0.05"), risk.MaintMargin)
	assert.Equal(t, fixedpoint.MustNewFromString("1"), risk.Adl)
	assert.Equal(t, fixedpoint.MustNewFromString("0.1"), risk.OpenOrderInitialMargin)
	assert.Equal(t, types.MillisecondTimestamp(time.Unix(1234567890/1000, 0)), risk.UpdateTime)
	assert.Equal(t, "USDT", risk.MarginAsset)
}

func TestToGlobalFuturesOrderType(t *testing.T) {
	tests := []struct {
		name      string
		orderType interface{} // futures.OrderType or futures.AlgoOrderType
		expected  types.OrderType
	}{
		// Test futures.OrderType values
		{
			name:      "OrderTypeLimit",
			orderType: futures.OrderTypeLimit,
			expected:  types.OrderTypeLimit,
		},
		{
			name:      "OrderTypeMarket",
			orderType: futures.OrderTypeMarket,
			expected:  types.OrderTypeMarket,
		},
		// Test futures.AlgoOrderType values
		{
			name:      "AlgoOrderTypeStop",
			orderType: futures.AlgoOrderTypeStop,
			expected:  types.OrderTypeStopLimit,
		},
		{
			name:      "AlgoOrderTypeStopMarket",
			orderType: futures.AlgoOrderTypeStopMarket,
			expected:  types.OrderTypeStopMarket,
		},
		{
			name:      "AlgoOrderTypeTakeProfit",
			orderType: futures.AlgoOrderTypeTakeProfit,
			expected:  types.OrderTypeTakeProfit,
		},
		{
			name:      "AlgoOrderTypeTakeProfitMarket",
			orderType: futures.AlgoOrderTypeTakeProfitMarket,
			expected:  types.OrderTypeTakeProfitMarket,
		},
		{
			name:      "AlgoOrderTypeTrailingStopMarket",
			orderType: futures.AlgoOrderTypeTrailingStopMarket,
			expected:  types.OrderTypeStopMarket,
		},
		// Test unsupported order type
		{
			name:      "UnsupportedOrderType",
			orderType: futures.OrderType("UNSUPPORTED"),
			expected:  "",
		},
		{
			name:      "UnsupportedAlgoOrderType",
			orderType: futures.AlgoOrderType("UNSUPPORTED"),
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result types.OrderType
			switch v := tt.orderType.(type) {
			case futures.OrderType:
				result = toGlobalFuturesOrderType(v)
			case futures.AlgoOrderType:
				result = toGlobalFuturesOrderType(v)
			default:
				t.Fatalf("unexpected type: %T", tt.orderType)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToGlobalFuturesOrder(t *testing.T) {
	t.Run("futures.Order", func(t *testing.T) {
		order := &futures.Order{
			Symbol:           "BTCUSDT",
			OrderID:          123456789,
			ClientOrderID:    "test-client-order-id",
			Price:            "50000",
			AvgPrice:         "49950",
			OrigQuantity:     "1.0",
			ExecutedQuantity: "0.5",
			Status:           futures.OrderStatusTypePartiallyFilled,
			TimeInForce:      futures.TimeInForceTypeGTC,
			Type:             futures.OrderTypeLimit,
			Side:             futures.SideTypeBuy,
			ReduceOnly:       false,
			ClosePosition:    false,
			StopPrice:        "49000",
			Time:             1609459200000, // 2021-01-01 00:00:00 UTC in milliseconds
			UpdateTime:       1609459260000, // 2021-01-01 00:01:00 UTC in milliseconds
		}

		result, err := toGlobalFuturesOrder(order, false)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		assert.Equal(t, types.ExchangeBinance, result.Exchange)
		assert.Equal(t, uint64(123456789), result.OrderID)
		assert.Equal(t, "test-client-order-id", result.ClientOrderID)
		assert.Equal(t, "BTCUSDT", result.Symbol)
		assert.Equal(t, types.SideTypeBuy, result.Side)
		assert.Equal(t, types.OrderTypeLimit, result.Type)
		assert.Equal(t, fixedpoint.MustNewFromString("1.0"), result.Quantity)
		assert.Equal(t, fixedpoint.MustNewFromString("0.5"), result.ExecutedQuantity)
		assert.Equal(t, fixedpoint.MustNewFromString("50000"), result.Price)
		assert.Equal(t, fixedpoint.MustNewFromString("49000"), result.StopPrice)
		assert.Equal(t, types.TimeInForce("GTC"), result.TimeInForce)
		assert.Equal(t, types.OrderStatusPartiallyFilled, result.Status)
		assert.False(t, result.ReduceOnly)
		assert.False(t, result.ClosePosition)
		assert.True(t, result.IsFutures)
		assert.False(t, result.IsIsolated)
	})

	t.Run("futures.Order with empty price uses avgPrice", func(t *testing.T) {
		order := &futures.Order{
			Symbol:           "ETHUSDT",
			OrderID:          987654321,
			ClientOrderID:    "test-client-order-id-2",
			Price:            "",
			AvgPrice:         "3000",
			OrigQuantity:     "2.0",
			ExecutedQuantity: "0",
			Status:           futures.OrderStatusTypeNew,
			TimeInForce:      futures.TimeInForceTypeGTC,
			Type:             futures.OrderTypeMarket,
			Side:             futures.SideTypeSell,
			ReduceOnly:       true,
			ClosePosition:    false,
			StopPrice:        "",
			Time:             1609459200000,
			UpdateTime:       1609459200000,
		}

		result, err := toGlobalFuturesOrder(order, true)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, fixedpoint.MustNewFromString("3000"), result.Price)
		assert.True(t, result.IsIsolated)
		assert.True(t, result.ReduceOnly)
	})

	t.Run("futures.CreateAlgoOrderResp", func(t *testing.T) {
		algoOrder := &futures.CreateAlgoOrderResp{
			Symbol:        "BTCUSDT",
			AlgoId:        987654321,
			ClientAlgoId:  "test-algo-order-id",
			Price:         "50000",
			Quantity:      "1.0",
			OrderType:     futures.AlgoOrderTypeStop,
			Side:          futures.SideTypeBuy,
			ReduceOnly:    false,
			ClosePosition: false,
			TriggerPrice:  "49000",
			TimeInForce:   futures.TimeInForceTypeGTC,
			AlgoStatus:    "NEW",
			CreateTime:    1609459200000,
			UpdateTime:    1609459260000,
		}

		result, err := toGlobalFuturesOrder(algoOrder, false)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		assert.Equal(t, types.ExchangeBinance, result.Exchange)
		assert.Equal(t, uint64(987654321), result.OrderID)
		assert.Equal(t, "test-algo-order-id", result.ClientOrderID)
		assert.Equal(t, "BTCUSDT", result.Symbol)
		assert.Equal(t, types.SideTypeBuy, result.Side)
		assert.Equal(t, types.OrderTypeStopLimit, result.Type)
		assert.Equal(t, fixedpoint.MustNewFromString("1.0"), result.Quantity)
		assert.Equal(t, fixedpoint.MustNewFromString("0"), result.ExecutedQuantity) // CreateAlgoOrderResp doesn't have ExecutedQuantity
		assert.Equal(t, fixedpoint.MustNewFromString("50000"), result.Price)
		assert.Equal(t, fixedpoint.MustNewFromString("49000"), result.StopPrice)
		assert.Equal(t, types.TimeInForce("GTC"), result.TimeInForce)
		assert.False(t, result.ReduceOnly)
		assert.False(t, result.ClosePosition)
		assert.True(t, result.IsFutures)
		assert.False(t, result.IsIsolated)
	})

	t.Run("futures.CreateAlgoOrderResp with AlgoOrderTypeTakeProfitMarket", func(t *testing.T) {
		algoOrder := &futures.CreateAlgoOrderResp{
			Symbol:        "ETHUSDT",
			AlgoId:        111222333,
			ClientAlgoId:  "test-algo-tp-market",
			Price:         "3100",
			Quantity:      "5.0",
			OrderType:     futures.AlgoOrderTypeTakeProfitMarket,
			Side:          futures.SideTypeSell,
			ReduceOnly:    true,
			ClosePosition: true,
			TriggerPrice:  "3200",
			TimeInForce:   futures.TimeInForceTypeGTC,
			AlgoStatus:    "NEW",
			CreateTime:    1609459200000,
			UpdateTime:    1609459200000,
		}

		result, err := toGlobalFuturesOrder(algoOrder, true)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, types.OrderTypeTakeProfitMarket, result.Type)
		assert.True(t, result.ReduceOnly)
		assert.True(t, result.ClosePosition)
		assert.True(t, result.IsIsolated)
	})

}
