package core

import "github.com/c9s/bbgo/pkg/types"

type Converter interface {
	OrderConverter
	TradeConverter
}

// OrderConverter converts the order to another order
type OrderConverter interface {
	ConvertOrder(order types.Order) (types.Order, error)
}

// TradeConverter converts the trade to another trade
type TradeConverter interface {
	ConvertTrade(trade types.Trade) (types.Trade, error)
}

// SymbolConverter converts the symbol to another symbol
type SymbolConverter struct {
	fromSymbol, toSymbol string
}

func NewSymbolConverter(fromSymbol, toSymbol string) *SymbolConverter {
	return &SymbolConverter{fromSymbol: fromSymbol, toSymbol: toSymbol}
}

func (c *SymbolConverter) ConvertOrder(order types.Order) (types.Order, error) {
	if order.Symbol == c.fromSymbol {
		order.Symbol = c.toSymbol
	}

	return order, nil
}

func (c *SymbolConverter) ConvertTrade(trade types.Trade) (types.Trade, error) {
	if trade.Symbol == c.fromSymbol {
		trade.Symbol = c.toSymbol
	}

	return trade, nil
}
