package testhelper

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var markets = map[string]types.Market{
	"BTCUSDT": {
		Symbol:          "BTCUSDT",
		PricePrecision:  2,
		VolumePrecision: 8,
		QuoteCurrency:   "USDT",
		BaseCurrency:    "BTC",
		MinNotional:     fixedpoint.MustNewFromString("0.001"),
		MinAmount:       fixedpoint.MustNewFromString("10.0"),
		MinQuantity:     fixedpoint.MustNewFromString("0.001"),
		TickSize:        fixedpoint.MustNewFromString("0.01"),
	},

	"ETHUSDT": {
		Symbol:          "ETH",
		PricePrecision:  2,
		VolumePrecision: 8,
		QuoteCurrency:   "USDT",
		BaseCurrency:    "ETH",
		MinNotional:     fixedpoint.MustNewFromString("0.005"),
		MinAmount:       fixedpoint.MustNewFromString("10.0"),
		MinQuantity:     fixedpoint.MustNewFromString("0.001"),
		TickSize:        fixedpoint.MustNewFromString("0.01"),
	},
}

func Market(symbol string) types.Market {
	market, ok := markets[symbol]
	if !ok {
		panic(fmt.Errorf("%s market not found, valid markets: %+v", symbol, markets))
	}

	return market
}
