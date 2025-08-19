package core

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

// pkg/core/tradecollector_test.go
func TestInitialize_ValidCurrencies(t *testing.T) {
	converter := NewCurrencyConverter("MAXEXCHANGE", "MAX")
	err := converter.Initialize()
	assert.NoError(t, err)
}

func TestInitialize_EmptyFromCurrency(t *testing.T) {
	converter := NewCurrencyConverter("", "MAX")
	err := converter.Initialize()
	assert.Error(t, err)
	assert.Equal(t, "FromCurrency can not be empty", err.Error())
}

func TestInitialize_EmptyToCurrency(t *testing.T) {
	converter := NewCurrencyConverter("MAXEXCHANGE", "")
	err := converter.Initialize()
	assert.Error(t, err)
	assert.Equal(t, "ToCurrency can not be empty", err.Error())
}

func TestConvertOrder_ValidConversion(t *testing.T) {
	converter := NewCurrencyConverter("MAXEXCHANGE", "MAX")
	order := types.Order{
		SubmitOrder: types.SubmitOrder{
			Market: types.Market{
				QuoteCurrency: "MAXEXCHANGE",
				BaseCurrency:  "MAXEXCHANGE",
			},
		},
	}
	convertedOrder, err := converter.ConvertOrder(order)
	assert.NoError(t, err)
	assert.Equal(t, "MAX", convertedOrder.SubmitOrder.Market.QuoteCurrency)
	assert.Equal(t, "MAX", convertedOrder.SubmitOrder.Market.BaseCurrency)
}

func TestConvertOrder_NoConversion(t *testing.T) {
	converter := NewCurrencyConverter("MAXEXCHANGE", "MAX")
	order := types.Order{
		SubmitOrder: types.SubmitOrder{
			Market: types.Market{
				QuoteCurrency: "JPY",
				BaseCurrency:  "JPY",
			},
		},
	}
	convertedOrder, err := converter.ConvertOrder(order)
	assert.NoError(t, err)
	assert.Equal(t, "JPY", convertedOrder.SubmitOrder.Market.QuoteCurrency)
	assert.Equal(t, "JPY", convertedOrder.SubmitOrder.Market.BaseCurrency)
}

func TestConvertTrade_ValidConversion(t *testing.T) {
	converter := NewCurrencyConverter("MAXEXCHANGE", "MAX")
	trade := types.Trade{
		FeeCurrency: "MAXEXCHANGE",
	}
	convertedTrade, err := converter.ConvertTrade(trade)
	assert.NoError(t, err)
	assert.Equal(t, "MAX", convertedTrade.FeeCurrency)
}

func TestConvertTrade_NoConversion(t *testing.T) {
	converter := NewCurrencyConverter("MAXEXCHANGE", "MAX")
	trade := types.Trade{
		FeeCurrency: "JPY",
	}
	convertedTrade, err := converter.ConvertTrade(trade)
	assert.NoError(t, err)
	assert.Equal(t, "JPY", convertedTrade.FeeCurrency)
}

func TestConvertMarket_ValidConversion(t *testing.T) {
	converter := NewCurrencyConverter("MAXEXCHANGE", "MAX")
	market := types.Market{
		QuoteCurrency: "MAXEXCHANGE",
		BaseCurrency:  "MAXEXCHANGE",
	}
	convertedMarket, err := converter.ConvertMarket(market)
	assert.NoError(t, err)
	assert.Equal(t, "MAX", convertedMarket.QuoteCurrency)
	assert.Equal(t, "MAX", convertedMarket.BaseCurrency)
}

func TestConvertMarket_NoConversion(t *testing.T) {
	converter := NewCurrencyConverter("MAXEXCHANGE", "MAX")
	market := types.Market{
		QuoteCurrency: "JPY",
		BaseCurrency:  "JPY",
	}
	convertedMarket, err := converter.ConvertMarket(market)
	assert.NoError(t, err)
	assert.Equal(t, "JPY", convertedMarket.QuoteCurrency)
	assert.Equal(t, "JPY", convertedMarket.BaseCurrency)
}

func TestConvertBalance_ValidConversion(t *testing.T) {
	converter := NewCurrencyConverter("MAXEXCHANGE", "MAX")
	balance := types.Balance{
		Currency: "MAXEXCHANGE",
	}
	convertedBalance, err := converter.ConvertBalance(balance)
	assert.NoError(t, err)
	assert.Equal(t, "MAX", convertedBalance.Currency)
}

func TestConvertBalance_NoConversion(t *testing.T) {
	converter := NewCurrencyConverter("MAXEXCHANGE", "MAX")
	balance := types.Balance{
		Currency: "JPY",
	}
	convertedBalance, err := converter.ConvertBalance(balance)
	assert.NoError(t, err)
	assert.Equal(t, "JPY", convertedBalance.Currency)
}
