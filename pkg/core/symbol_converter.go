package core

import (
	"errors"

	"github.com/c9s/bbgo/pkg/types"
)

type Converter interface {
	OrderConverter
	TradeConverter
	KLineConverter
	MarketConverter
	BalanceConverter
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

// KLineConverter converts the kline to another kline
type KLineConverter interface {
	ConvertKLine(kline types.KLine) (types.KLine, error)
}

// MarketConverter converts the market to another market
type MarketConverter interface {
	ConvertMarket(market types.Market) (types.Market, error)
}

// BalanceConverter converts the balance to another balance
type BalanceConverter interface {
	ConvertBalance(balance types.Balance) (types.Balance, error)
}

type OrderConvertFunc func(order types.Order) (types.Order, error)
type TradeConvertFunc func(trade types.Trade) (types.Trade, error)
type KLineConvertFunc func(kline types.KLine) (types.KLine, error)
type MarketConvertFunc func(market types.Market) (types.Market, error)
type BalanceConvertFunc func(balance types.Balance) (types.Balance, error)

type DynamicConverter struct {
	orderConverter   OrderConvertFunc
	tradeConverter   TradeConvertFunc
	klineConverter   KLineConvertFunc
	marketConverter  MarketConvertFunc
	balanceConverter BalanceConvertFunc
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

func (c *DynamicConverter) ConvertKLine(kline types.KLine) (types.KLine, error) {
	return c.klineConverter(kline)
}

func (c *DynamicConverter) ConvertMarket(market types.Market) (types.Market, error) {
	return c.marketConverter(market)
}

func (c *DynamicConverter) ConvertBalance(balance types.Balance) (types.Balance, error) {
	return c.balanceConverter(balance)
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

	if order.SubmitOrder.Market.Symbol == c.FromSymbol {
		order.SubmitOrder.Market.Symbol = c.ToSymbol
	}

	return order, nil
}

func (c *SymbolConverter) ConvertTrade(trade types.Trade) (types.Trade, error) {
	if trade.Symbol == c.FromSymbol {
		trade.Symbol = c.ToSymbol
	}

	return trade, nil
}

func (c *SymbolConverter) ConvertKLine(kline types.KLine) (types.KLine, error) {
	if kline.Symbol == c.FromSymbol {
		kline.Symbol = c.ToSymbol
	}

	return kline, nil
}

func (s *SymbolConverter) ConvertMarket(mkt types.Market) (types.Market, error) {
	if mkt.Symbol == s.FromSymbol {
		mkt.Symbol = s.ToSymbol
	}
	return mkt, nil
}

func (c *SymbolConverter) ConvertBalance(balance types.Balance) (types.Balance, error) {
	return balance, nil
}
