package testhelper

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var _markets = types.MarketMap{
	"BTCTWD": {
		Symbol:          "BTCTWD",
		PricePrecision:  0,
		VolumePrecision: 2,
		QuoteCurrency:   "TWD",
		BaseCurrency:    "BTC",
		MinNotional:     fixedpoint.MustNewFromString("300.0"),
		MinAmount:       fixedpoint.MustNewFromString("300.0"),
		MinQuantity:     fixedpoint.MustNewFromString("0.001"),
		TickSize:        fixedpoint.MustNewFromString("1"),
		StepSize:        Number(0.0001),
	},
	"BTCUSDT": {
		Symbol:          "BTCUSDT",
		PricePrecision:  2,
		VolumePrecision: 8,
		QuoteCurrency:   "USDT",
		BaseCurrency:    "BTC",
		MinNotional:     fixedpoint.MustNewFromString("10.0"),
		MinAmount:       fixedpoint.MustNewFromString("10.0"),
		MinQuantity:     fixedpoint.MustNewFromString("0.001"),
		TickSize:        fixedpoint.MustNewFromString("0.01"),
		StepSize:        Number(0.0001),
	},

	"ETHUSDT": {
		Symbol:          "ETHUSDT",
		PricePrecision:  2,
		VolumePrecision: 8,
		QuoteCurrency:   "USDT",
		BaseCurrency:    "ETH",
		MinNotional:     fixedpoint.MustNewFromString("10.0"),
		MinAmount:       fixedpoint.MustNewFromString("10.0"),
		MinQuantity:     fixedpoint.MustNewFromString("0.001"),
		TickSize:        fixedpoint.MustNewFromString("0.01"),
		StepSize:        Number(0.0001),
	},

	"USDCUSDT": {
		Symbol:          "USDCUSDT",
		PricePrecision:  5,
		VolumePrecision: 2,
		QuoteCurrency:   "USDT",
		BaseCurrency:    "USDC",
		MinNotional:     fixedpoint.MustNewFromString("10.0"),
		MinAmount:       fixedpoint.MustNewFromString("10.0"),
		MinQuantity:     fixedpoint.MustNewFromString("10.0"),
		TickSize:        fixedpoint.MustNewFromString("0.0001"),
		StepSize:        Number(1.0),
	},

	"USDTTWD": {
		Symbol:          "USDTTWD",
		PricePrecision:  2,
		VolumePrecision: 1,
		QuoteCurrency:   "TWD",
		BaseCurrency:    "USDT",
		MinNotional:     fixedpoint.MustNewFromString("10.0"),
		MinAmount:       fixedpoint.MustNewFromString("10.0"),
		MinQuantity:     fixedpoint.MustNewFromString("10.0"),
		TickSize:        fixedpoint.MustNewFromString("0.01"),
		StepSize:        Number(1.0),
	},
}

func AllMarkets() types.MarketMap {
	return _markets
}

func Market(symbol string) types.Market {
	market, ok := _markets[symbol]
	if !ok {
		panic(fmt.Errorf("%s test market not found, valid markets: %+v", symbol, _markets))
	}

	return market
}
