package core

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

type ConverterSetting struct {
	SymbolConverter   *SymbolConverter   `json:"symbolConverter" yaml:"symbolConverter"`
	CurrencyConverter *CurrencyConverter `json:"currencyConverter" yaml:"currencyConverter"`
}

func (s *ConverterSetting) getConverter() Converter {
	if s.SymbolConverter != nil {
		return s.SymbolConverter
	}

	if s.CurrencyConverter != nil {
		return s.CurrencyConverter
	}

	return nil
}

func (s *ConverterSetting) InitializeConverter() (Converter, error) {
	converter := s.getConverter()
	if converter == nil {
		return nil, nil
	}

	logrus.Infof("initializing converter %T ...", converter)
	err := converter.Initialize()
	return converter, err
}

// ConverterManager manages the converters for trade conversion
// It can be used to convert the trade symbol into the target symbol, or convert the price, volume into different units.
type ConverterManager struct {
	ConverterSettings []ConverterSetting `json:"converters,omitempty" yaml:"converters,omitempty"`

	converters []Converter
}

func (c *ConverterManager) Initialize() error {
	for _, setting := range c.ConverterSettings {
		converter, err := setting.InitializeConverter()
		if err != nil {
			return err
		}

		if converter != nil {
			c.AddConverter(converter)
		}
	}

	numConverters := len(c.converters)
	logrus.Infof("%d converters loaded", numConverters)
	return nil
}

func (c *ConverterManager) AddConverter(converter Converter) {
	c.converters = append(c.converters, converter)
}

func (c *ConverterManager) ConvertOrder(order types.Order) types.Order {
	if len(c.converters) == 0 {
		return order
	}

	for _, converter := range c.converters {
		convOrder, err := converter.ConvertOrder(order)
		if err != nil {
			logrus.WithError(err).Errorf("converter %+v error, order: %s", converter, order.String())
			continue
		}

		order = convOrder
	}

	return order
}

func (c *ConverterManager) ConvertTrade(trade types.Trade) types.Trade {
	if len(c.converters) == 0 {
		return trade
	}

	for _, converter := range c.converters {
		convTrade, err := converter.ConvertTrade(trade)
		if err != nil {
			logrus.WithError(err).Errorf("converter %+v error, trade: %s", converter, trade.String())
			continue
		}

		trade = convTrade
	}

	return trade
}

func (c *ConverterManager) ConvertKLine(kline types.KLine) types.KLine {
	if len(c.converters) == 0 {
		return kline
	}

	for _, converter := range c.converters {
		convKline, err := converter.ConvertKLine(kline)
		if err != nil {
			logrus.WithError(err).Errorf("converter %+v error, kline: %s", converter, kline.String())
			continue
		}

		kline = convKline
	}

	return kline
}

func (c *ConverterManager) ConvertMarket(market types.Market) types.Market {
	if len(c.converters) == 0 {
		return market
	}

	for _, converter := range c.converters {
		convMarket, err := converter.ConvertMarket(market)
		if err != nil {
			logrus.WithError(err).Errorf("converter %+v error, market: %+v", converter, market)
			continue
		}

		market = convMarket
	}

	return market
}

func (c *ConverterManager) ConvertBalance(balance types.Balance) types.Balance {
	if len(c.converters) == 0 {
		return balance
	}

	for _, converter := range c.converters {
		convBal, err := converter.ConvertBalance(balance)
		if err != nil {
			logrus.WithError(err).Errorf("converter %+v error, balance: %s", converter, balance.String())
			continue
		}

		balance = convBal
	}

	return balance
}
