package types

import (
	"math"
	"strconv"
)

type Market struct {
	Symbol          string
	PricePrecision  int
	VolumePrecision int
	QuoteCurrency   string
	BaseCurrency    string
	MinQuantity     float64
	MinAmount       float64
	MinLot          float64
}

func (m Market) FormatPrice(val float64) string {
	return strconv.FormatFloat(val, 'f', m.PricePrecision, 64)
}

func (m Market) FormatVolume(val float64) string {
	return strconv.FormatFloat(val, 'f', m.VolumePrecision, 64)
}

func (m Market) CanonicalizeVolume(val float64) float64 {
	p := math.Pow10(m.VolumePrecision)
	return math.Trunc(p*val) / p
}

var MarketBTCUSDT = Market{
	Symbol:          "BTCUSDT",
	BaseCurrency:    "BTC",
	QuoteCurrency:   "USDT",
	PricePrecision:  2,
	VolumePrecision: 6,
	MinQuantity:     0.000001,
	MinAmount:       10.0,
	MinLot:          0.000001,
}

var MarketETHUSDT = Market{
	Symbol:          "ETHUSDT",
	BaseCurrency:    "ETH",
	QuoteCurrency:   "USDT",
	PricePrecision:  2,
	VolumePrecision: 5,
	MinQuantity:     0.01,
	MinAmount:       10.0,
	MinLot:          0.01,
}


var MarketBNBUSDT = Market{
	Symbol:          "BNBUSDT",
	BaseCurrency:    "BNB",
	QuoteCurrency:   "USDT",
	PricePrecision:  4,
	VolumePrecision: 2,
	MinQuantity:     0.01,
	MinAmount:       10.0,
	MinLot:          0.01,
}

var Markets = map[string]Market{
	"ETHUSDT": MarketETHUSDT,
	"BNBUSDT": MarketBNBUSDT,
	"BTCUSDT": MarketBTCUSDT,
}

func FindMarket(symbol string) (m Market, ok bool) {
	m, ok = Markets[symbol]
	return m, ok
}
