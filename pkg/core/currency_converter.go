package core

import (
	"errors"
	"github.com/c9s/bbgo/pkg/types"
)

type CurrencyConverter struct {
	FromCurrency string `json:"from"`
	ToCurrency   string `json:"to"`
}

func NewCurrencyConverter(fromSymbol, toSymbol string) *CurrencyConverter {
	return &CurrencyConverter{FromCurrency: fromSymbol, ToCurrency: toSymbol}
}

func (c *CurrencyConverter) Initialize() error {
	if c.FromCurrency == "" {
		return errors.New("FromCurrency can not be empty")
	}

	if c.ToCurrency == "" {
		return errors.New("ToCurrency can not be empty")
	}

	return nil
}

func (c *CurrencyConverter) ConvertOrder(order types.Order) (types.Order, error) {
	if order.SubmitOrder.Market.QuoteCurrency == c.FromCurrency {
		order.SubmitOrder.Market.QuoteCurrency = c.ToCurrency
	}
	if order.SubmitOrder.Market.BaseCurrency == c.FromCurrency {
		order.SubmitOrder.Market.BaseCurrency = c.ToCurrency
	}

	return order, nil
}

func (c *CurrencyConverter) ConvertTrade(trade types.Trade) (types.Trade, error) {
	if trade.FeeCurrency == c.FromCurrency {
		trade.FeeCurrency = c.ToCurrency
	}
	return trade, nil
}

func (c *CurrencyConverter) ConvertKLine(kline types.KLine) (types.KLine, error) {
	return kline, nil
}

func (c *CurrencyConverter) ConvertMarket(mkt types.Market) (types.Market, error) {
	if mkt.QuoteCurrency == c.FromCurrency {
		mkt.QuoteCurrency = c.ToCurrency
	}
	if mkt.BaseCurrency == c.FromCurrency {
		mkt.BaseCurrency = c.ToCurrency
	}

	return mkt, nil
}

func (c *CurrencyConverter) ConvertBalance(balance types.Balance) (types.Balance, error) {
	if balance.Currency == c.FromCurrency {
		balance.Currency = c.ToCurrency
	}

	return balance, nil
}
