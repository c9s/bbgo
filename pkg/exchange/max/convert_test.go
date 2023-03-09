package max

import (
	"encoding/json"
	"testing"

	v3 "github.com/c9s/bbgo/pkg/exchange/max/maxapi/v3"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_toGlobalTradeV3(t *testing.T) {
	assert := assert.New(t)

	t.Run("ask trade", func(t *testing.T) {
		str := `
		{
			"id": 68444,
			"order_id": 87,
			"wallet_type": "spot",
			"price": "21499.0",
			"volume": "0.2658",
			"funds": "5714.4",
			"market": "ethtwd",
			"market_name": "ETH/TWD",
			"side": "bid",
			"fee": "0.00001",
			"fee_currency": "usdt",
			"self_trade_bid_fee": "0.00001",
			"self_trade_bid_fee_currency": "eth",
			"self_trade_bid_order_id": 86,
			"liquidity": "maker",
			"created_at": 1521726960357
		}
		`

		var trade v3.Trade
		assert.NoError(json.Unmarshal([]byte(str), &trade))

		trades, err := toGlobalTradeV3(trade)
		assert.NoError(err)
		assert.Len(trades, 1)

		assert.Equal(uint64(87), trades[0].OrderID)
		assert.Equal(types.SideTypeBuy, trades[0].Side)
	})

	t.Run("bid trade", func(t *testing.T) {
		str := `
		{
			"id": 68444,
			"order_id": 87,
			"wallet_type": "spot",
			"price": "21499.0",
			"volume": "0.2658",
			"funds": "5714.4",
			"market": "ethtwd",
			"market_name": "ETH/TWD",
			"side": "ask",
			"fee": "0.00001",
			"fee_currency": "usdt",
			"self_trade_bid_fee": "0.00001",
			"self_trade_bid_fee_currency": "eth",
			"self_trade_bid_order_id": 86,
			"liquidity": "maker",
			"created_at": 1521726960357
		}
		`

		var trade v3.Trade
		assert.NoError(json.Unmarshal([]byte(str), &trade))

		trades, err := toGlobalTradeV3(trade)
		assert.NoError(err)
		assert.Len(trades, 1)

		assert.Equal(uint64(87), trades[0].OrderID)
		assert.Equal(types.SideTypeSell, trades[0].Side)
	})

	t.Run("self trade", func(t *testing.T) {
		str := `
		{
			"id": 68444,
			"order_id": 87,
			"wallet_type": "spot",
			"price": "21499.0",
			"volume": "0.2658",
			"funds": "5714.4",
			"market": "ethtwd",
			"market_name": "ETH/TWD",
			"side": "self-trade",
			"fee": "0.00001",
			"fee_currency": "usdt",
			"self_trade_bid_fee": "0.00001",
			"self_trade_bid_fee_currency": "eth",
			"self_trade_bid_order_id": 86,
			"liquidity": "maker",
			"created_at": 1521726960357
		}
		`

		var trade v3.Trade
		assert.NoError(json.Unmarshal([]byte(str), &trade))

		trades, err := toGlobalTradeV3(trade)
		assert.NoError(err)
		assert.Len(trades, 2)

		assert.Equal(uint64(86), trades[0].OrderID)
		assert.Equal(types.SideTypeBuy, trades[0].Side)

		assert.Equal(uint64(87), trades[1].OrderID)
		assert.Equal(types.SideTypeSell, trades[1].Side)
	})
}
