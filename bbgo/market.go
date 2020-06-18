package bbgo

import "strconv"

type Market struct {
	Symbol          string
	PricePrecision  int
	VolumePrecision int
}

func (m Market) FormatPrice(val float64) string {
	return strconv.FormatFloat(val, 'f', m.PricePrecision, 64)
}

func (m Market) FormatVolume(val float64) string {
	return strconv.FormatFloat(val, 'f', m.VolumePrecision, 64)
}


//  Binance Markets, this should be defined per exchange
var Markets = map[string]Market{
	"BNBUSDT": {
		Symbol:          "BNBUSDT",
		PricePrecision:  4,
		VolumePrecision: 2,
	},
	"BTCUSDT": {
		Symbol:          "BTCUSDT",
		PricePrecision:  2,
		VolumePrecision: 8,
	},
}

func FindMarket(symbol string) (m  Market, ok bool) {
	m , ok = Markets[symbol]
	return m, ok
}


