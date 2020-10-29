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

	// The MIN_NOTIONAL filter defines the minimum notional value allowed for an order on a symbol. An order's notional value is the price * quantity
	MinNotional float64
	MinAmount   float64

	// The LOT_SIZE filter defines the quantity
	MinLot      float64
	MinQuantity float64
	MaxQuantity float64

	MinPrice float64
	MaxPrice float64
	TickSize float64
}

func (m Market) FormatPriceCurrency(val float64) string {
	switch m.QuoteCurrency {

	case "USD", "USDT":
		return USD.FormatMoneyFloat64(val)

	case "BTC":
		return BTC.FormatMoneyFloat64(val)

	case "BNB":
		return BNB.FormatMoneyFloat64(val)

	}

	return m.FormatPrice(val)
}

func (m Market) FormatPrice(val float64) string {
	// p := math.Pow10(m.PricePrecision)

	prec := int(math.Abs(math.Log10(m.MinPrice)))
	p := math.Pow10(prec)
	val = math.Trunc(val*p) / p
	return strconv.FormatFloat(val, 'f', m.PricePrecision, 64)
}

func (m Market) FormatQuantity(val float64) string {
	prec := int(math.Abs(math.Log10(m.MinLot)))
	p := math.Pow10(prec)
	val = math.Trunc(val*p) / p
	return strconv.FormatFloat(val, 'f', prec, 64)
}

func (m Market) FormatVolume(val float64) string {
	p := math.Pow10(m.VolumePrecision)
	val = math.Trunc(val*p) / p
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
	MinLot:          0.000001,
	MinAmount:       10.0,
	MinNotional:     10.0,
}

var MarketETHUSDT = Market{
	Symbol:          "ETHUSDT",
	BaseCurrency:    "ETH",
	QuoteCurrency:   "USDT",
	PricePrecision:  2,
	VolumePrecision: 5,
	MinQuantity:     0.01,
	MinLot:          0.01,
	MinAmount:       10.0,
	MinNotional:     10.0,
}

var MarketBNBUSDT = Market{
	Symbol:          "BNBUSDT",
	BaseCurrency:    "BNB",
	QuoteCurrency:   "USDT",
	PricePrecision:  4,
	VolumePrecision: 2,
	MinQuantity:     0.01,
	MinLot:          0.01,
	MinAmount:       10.0,
	MinNotional:     10.0,
}

type MarketMap map[string]Market

var Markets = map[string]Market{
	"ETHUSDT": MarketETHUSDT,
	"BNBUSDT": MarketBNBUSDT,
	"BTCUSDT": MarketBTCUSDT,
}

func FindMarket(symbol string) (m Market, ok bool) {
	m, ok = Markets[symbol]
	return m, ok
}
