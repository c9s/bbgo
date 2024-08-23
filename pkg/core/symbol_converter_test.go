package core

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestSymbolConverter(t *testing.T) {
	converter := NewSymbolConverter("MAXEXCHANGEUSDT", "MAXUSDT")
	trade, err := converter.ConvertTrade(types.Trade{
		Symbol: "MAXEXCHANGEUSDT",
	})

	if assert.NoError(t, err) {
		assert.Equal(t, "MAXUSDT", trade.Symbol)
	}

	order, err := converter.ConvertOrder(types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol: "MAXEXCHANGEUSDT",
			Market: types.Market{
				Symbol: "MAXEXCHANGEUSDT",
			},
		},
	})

	if assert.NoError(t, err) {
		assert.Equal(t, "MAXUSDT", order.Symbol)
		assert.Equal(t, "MAXUSDT", order.SubmitOrder.Symbol)
		assert.Equal(t, "MAXUSDT", order.SubmitOrder.Market.Symbol)
	}

	kline, err := converter.ConvertKLine(types.KLine{
		Symbol: "MAXEXCHANGEUSDT",
	})

	if assert.NoError(t, err) {
		assert.Equal(t, "MAXUSDT", kline.Symbol)
	}

	market, err := converter.ConvertMarket(types.Market{
		Symbol: "MAXEXCHANGEUSDT",
	})

	if assert.NoError(t, err) {
		assert.Equal(t, "MAXUSDT", market.Symbol)
	}

	balance, err := converter.ConvertBalance(types.Balance{
		Currency: "MAXEXCHANGE",
	})

	if assert.NoError(t, err) {
		assert.Equal(t, "MAXEXCHANGE", balance.Currency)
	}

}
