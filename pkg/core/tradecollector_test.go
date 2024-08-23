package core

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestInitializeConverter_ValidSymbolConverter(t *testing.T) {
	setting := ConverterSetting{
		SymbolConverter: &SymbolConverter{
			FromSymbol: "MAXEXCHANGEUSDT",
			ToSymbol:   "MAXUSDT",
		},
	}
	converter, err := setting.InitializeConverter()
	assert.NoError(t, err)
	assert.NotNil(t, converter)
}

func TestInitializeConverter_ValidCurrencyConverter(t *testing.T) {
	setting := ConverterSetting{
		CurrencyConverter: &CurrencyConverter{
			FromCurrency: "MAXEXCHANGE",
			ToCurrency:   "MAX",
		},
	}
	converter, err := setting.InitializeConverter()
	assert.NoError(t, err)
	assert.NotNil(t, converter)
}

func TestInitializeConverter_NoConverter(t *testing.T) {
	setting := ConverterSetting{}
	converter, err := setting.InitializeConverter()
	assert.NoError(t, err)
	assert.Nil(t, converter)
}

func TestInitialize_ValidConverters(t *testing.T) {
	manager := ConverterManager{
		ConverterSettings: []ConverterSetting{
			{SymbolConverter: &SymbolConverter{
				FromSymbol: "MAXEXCHANGEUSDT",
				ToSymbol:   "MAXUSDT",
			}},
			{CurrencyConverter: &CurrencyConverter{
				FromCurrency: "MAXEXCHANGE",
				ToCurrency:   "MAX",
			}},
		},
	}
	err := manager.Initialize()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(manager.converters))
}

func TestInitialize_NoConverters(t *testing.T) {
	manager := ConverterManager{}
	err := manager.Initialize()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(manager.converters))
}

func TestConvertOrder_WithConverters(t *testing.T) {
	manager := ConverterManager{
		ConverterSettings: []ConverterSetting{
			{SymbolConverter: &SymbolConverter{
				FromSymbol: "MAXEXCHANGEUSDT",
				ToSymbol:   "MAXUSDT",
			}},
			{CurrencyConverter: &CurrencyConverter{
				FromCurrency: "MAXEXCHANGE",
				ToCurrency:   "MAX",
			}},
		},
	}

	order := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol: "MAXEXCHANGEUSDT",
			Market: types.Market{
				Symbol:        "MAXEXCHANGEUSDT",
				QuoteCurrency: "USDT",
				BaseCurrency:  "MAXEXCHANGE",
			},
		},
	}
	err := manager.Initialize()
	assert.NoError(t, err)
	convertedOrder := manager.ConvertOrder(order)
	assert.Equal(t, "MAXUSDT", convertedOrder.Symbol)
	assert.Equal(t, "MAX", convertedOrder.Market.BaseCurrency)
	assert.Equal(t, "USDT", convertedOrder.Market.QuoteCurrency)
	assert.Equal(t, "MAXUSDT", convertedOrder.Market.Symbol)
}

func TestConvertOrder_NoConverters(t *testing.T) {
	manager := ConverterManager{}
	order := types.Order{}
	err := manager.Initialize()
	assert.NoError(t, err)
	convertedOrder := manager.ConvertOrder(order)
	assert.Equal(t, order, convertedOrder)
}

func TestConvertTrade_WithConverters(t *testing.T) {
	manager := ConverterManager{}
	converter := &CurrencyConverter{
		FromCurrency: "MAXEXCHANGE",
		ToCurrency:   "MAX",
	}
	err := manager.Initialize()
	assert.NoError(t, err)
	manager.AddConverter(converter)

	trade := types.Trade{}
	convertedTrade := manager.ConvertTrade(trade)
	assert.Equal(t, trade, convertedTrade)
}

func TestConvertTrade_NoConverters(t *testing.T) {
	manager := ConverterManager{}
	trade := types.Trade{}
	err := manager.Initialize()
	assert.NoError(t, err)
	convertedTrade := manager.ConvertTrade(trade)
	assert.Equal(t, trade, convertedTrade)
}

func TestConvertKLine_WithConverters(t *testing.T) {
	manager := ConverterManager{}
	converter := &CurrencyConverter{
		FromCurrency: "MAXEXCHANGE",
		ToCurrency:   "MAX",
	}
	err := manager.Initialize()
	assert.NoError(t, err)
	manager.AddConverter(converter)

	kline := types.KLine{}
	convertedKline := manager.ConvertKLine(kline)
	assert.Equal(t, kline, convertedKline)
}

func TestConvertKLine_NoConverters(t *testing.T) {
	manager := ConverterManager{}
	kline := types.KLine{}
	err := manager.Initialize()
	assert.NoError(t, err)
	convertedKline := manager.ConvertKLine(kline)
	assert.Equal(t, kline, convertedKline)
}

func TestConvertMarket_WithConverters(t *testing.T) {
	manager := ConverterManager{}
	converter := &CurrencyConverter{
		FromCurrency: "MAXEXCHANGE",
		ToCurrency:   "MAX",
	}
	err := manager.Initialize()
	assert.NoError(t, err)
	manager.AddConverter(converter)

	market := types.Market{}
	convertedMarket := manager.ConvertMarket(market)
	assert.Equal(t, market, convertedMarket)
}

func TestConvertMarket_NoConverters(t *testing.T) {
	manager := ConverterManager{}
	market := types.Market{}
	err := manager.Initialize()
	assert.NoError(t, err)
	convertedMarket := manager.ConvertMarket(market)
	assert.Equal(t, market, convertedMarket)
}

func TestConvertBalance_WithConverters(t *testing.T) {
	manager := ConverterManager{}
	converter := &CurrencyConverter{
		FromCurrency: "MAXEXCHANGE",
		ToCurrency:   "MAX",
	}
	err := manager.Initialize()
	assert.NoError(t, err)
	manager.AddConverter(converter)

	balance := types.Balance{}
	convertedBalance := manager.ConvertBalance(balance)
	assert.Equal(t, balance, convertedBalance)
}

func TestConvertBalance_NoConverters(t *testing.T) {
	manager := ConverterManager{}
	balance := types.Balance{}
	err := manager.Initialize()
	assert.NoError(t, err)
	convertedBalance := manager.ConvertBalance(balance)
	assert.Equal(t, balance, convertedBalance)
}

func TestTradeCollector_NilConvertManager(t *testing.T) {
	symbol := "BTCUSDT"
	position := types.NewPosition(symbol, "BTC", "USDT")
	orderStore := NewOrderStore(symbol)
	collector := NewTradeCollector(symbol, position, orderStore)

	trade := types.Trade{
		ID:            1,
		OrderID:       399,
		Exchange:      types.ExchangeBinance,
		Price:         fixedpoint.NewFromInt(40000),
		Quantity:      fixedpoint.One,
		QuoteQuantity: fixedpoint.NewFromInt(40000),
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeBuy,
		IsBuyer:       true,
	}

	trade = collector.ConvertTrade(trade)
	assert.Equal(t, "BTCUSDT", trade.Symbol)
}

func TestTradeCollector_ShouldNotCountDuplicatedTrade(t *testing.T) {
	symbol := "BTCUSDT"
	position := types.NewPosition(symbol, "BTC", "USDT")
	orderStore := NewOrderStore(symbol)
	collector := NewTradeCollector(symbol, position, orderStore)
	assert.NotNil(t, collector)

	matched := collector.RecoverTrade(types.Trade{
		ID:            1,
		OrderID:       399,
		Exchange:      types.ExchangeBinance,
		Price:         fixedpoint.NewFromInt(40000),
		Quantity:      fixedpoint.One,
		QuoteQuantity: fixedpoint.NewFromInt(40000),
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeBuy,
		IsBuyer:       true,
	})
	assert.False(t, matched, "should be added to the trade store")
	assert.Equal(t, 1, len(collector.tradeStore.Trades()), "should have 1 trade in the trade store")

	orderStore.Add(types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Quantity: fixedpoint.One,
			Price:    fixedpoint.NewFromInt(40000),
		},
		Exchange:         types.ExchangeBinance,
		OrderID:          399,
		Status:           types.OrderStatusFilled,
		ExecutedQuantity: fixedpoint.One,
		IsWorking:        false,
	})

	matched = collector.Process()
	assert.True(t, matched)
	assert.Equal(t, 0, len(collector.tradeStore.Trades()), "the found trade should be removed from the trade store")

	matched = collector.ProcessTrade(types.Trade{
		ID:            1,
		OrderID:       399,
		Exchange:      types.ExchangeBinance,
		Price:         fixedpoint.NewFromInt(40000),
		Quantity:      fixedpoint.One,
		QuoteQuantity: fixedpoint.NewFromInt(40000),
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeBuy,
		IsBuyer:       true,
	})
	assert.False(t, matched, "the same trade should not match")
	assert.Equal(t, 0, len(collector.tradeStore.Trades()), "the same trade should not be added to the trade store")
}
