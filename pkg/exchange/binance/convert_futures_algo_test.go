package binance

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/adshao/go-binance/v2/futures"

	"github.com/c9s/bbgo/pkg/types"
)

func TestToGlobalFuturesAlgoOrderConversion(t *testing.T) {
	resp := &futures.CreateAlgoOrderResp{
		ClientAlgoId:  "x-test-123",
		OrderType:     futures.AlgoOrderTypeStopMarket,
		Symbol:        "BTCUSDT",
		Side:          futures.SideTypeBuy,
		TimeInForce:   futures.TimeInForceTypeGTC,
		Quantity:      "0.001",
		TriggerPrice:  "50000",
		Price:         "",
		ReduceOnly:    true,
		ClosePosition: false,
		CreateTime:    1700000000000,
		UpdateTime:    1700000000000,
	}

	o, err := toGlobalFuturesAlgoOrder(resp)
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", o.Symbol)
	require.Equal(t, types.SideTypeBuy, o.Side)
	require.Equal(t, types.OrderTypeStopMarket, o.Type)
	require.Equal(t, "0.001", o.Quantity.String())
	require.Equal(t, "50000", o.StopPrice.String())
	require.Equal(t, "0", o.Price.String())
	require.True(t, o.IsFutures)
	require.Equal(t, types.OrderStatusNew, o.Status)
	require.Equal(t, "x-test-123", o.ClientOrderID)
}

func TestToLocalFuturesOrderTypeAlgoMapping(t *testing.T) {
	ot, err := toLocalFuturesOrderType(types.OrderTypeStopMarket)
	require.NoError(t, err)
	require.Equal(t, futures.OrderType("STOP_MARKET"), ot)

	ot2, err := toLocalFuturesOrderType(types.OrderTypeTakeProfitMarket)
	require.NoError(t, err)
	require.Equal(t, futures.OrderType("TAKE_PROFIT_MARKET"), ot2)
}
