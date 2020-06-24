package bbgo

import "strconv"

type Market struct {
	Symbol          string
	PricePrecision  int
	VolumePrecision int
	MinQuantity     float64
	MinAmount       float64
}

func (m Market) FormatPrice(val float64) string {
	return strconv.FormatFloat(val, 'f', m.PricePrecision, 64)
}

func (m Market) FormatVolume(val float64) string {
	return strconv.FormatFloat(val, 'f', m.VolumePrecision, 64)
}

//  Binance Markets, this should be defined per exchange

var MarketBTCUSDT = Market{
		Symbol:          "BTCUSDT",
		PricePrecision:  2,
		VolumePrecision: 6,
		MinQuantity:     0.00000100,
		MinAmount:       10.0,
}
var MarketBNBUSDT = Market{
		Symbol:          "BNBUSDT",
		PricePrecision:  4,
		VolumePrecision: 2,
		MinQuantity:     0.01,
		MinAmount:       10.0,
}

var Markets = map[string]Market{
	"BNBUSDT": MarketBNBUSDT,
	"BTCUSDT": MarketBTCUSDT,
}

func FindMarket(symbol string) (m Market, ok bool) {
	m, ok = Markets[symbol]
	return m, ok
}
