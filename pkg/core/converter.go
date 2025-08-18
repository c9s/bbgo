package core

import (
	"errors"

	"github.com/c9s/bbgo/pkg/types"
)

type Converter interface {
	OrderConverter
	TradeConverter
	Initialize() error
}

// OrderConverter converts the order to another order
type OrderConverter interface {
	ConvertOrder(order types.Order) (types.Order, error)
}

// TradeConverter converts the trade to another trade
type TradeConverter interface {
	ConvertTrade(trade types.Trade) (types.Trade, error)
}

type OrderConvertFunc func(order types.Order) (types.Order, error)
type TradeConvertFunc func(trade types.Trade) (types.Trade, error)

type DynamicConverter struct {
	orderConverter OrderConvertFunc
	tradeConverter TradeConvertFunc
}

func NewDynamicConverter(orderConverter OrderConvertFunc, tradeConverter TradeConvertFunc) *DynamicConverter {
	return &DynamicConverter{orderConverter: orderConverter, tradeConverter: tradeConverter}
}

func (c *DynamicConverter) Initialize() error {
	return nil
}

func (c *DynamicConverter) ConvertOrder(order types.Order) (types.Order, error) {
	return c.orderConverter(order)
}

func (c *DynamicConverter) ConvertTrade(trade types.Trade) (types.Trade, error) {
	return c.tradeConverter(trade)
}

// SymbolConverter converts the symbol to another symbol
type SymbolConverter struct {
	FromSymbol string `json:"from"`
	ToSymbol   string `json:"to"`
}

func NewSymbolConverter(fromSymbol, toSymbol string) *SymbolConverter {
	return &SymbolConverter{FromSymbol: fromSymbol, ToSymbol: toSymbol}
}

func (c *SymbolConverter) Initialize() error {
	if c.ToSymbol == "" {
		return errors.New("toSymbol can not be empty")
	}

	if c.FromSymbol == "" {
		return errors.New("fromSymbol can not be empty")
	}

	return nil
}

func (c *SymbolConverter) ConvertOrder(order types.Order) (types.Order, error) {
	if order.Symbol == c.FromSymbol {
		order.Symbol = c.ToSymbol
	}

	return order, nil
}

func (c *SymbolConverter) ConvertTrade(trade types.Trade) (types.Trade, error) {
	if trade.Symbol == c.FromSymbol {
		trade.Symbol = c.ToSymbol
	}

	return trade, nil
}
