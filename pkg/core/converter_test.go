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
		},
	})

	if assert.NoError(t, err) {
		assert.Equal(t, "MAXUSDT", order.Symbol)
	}

}
