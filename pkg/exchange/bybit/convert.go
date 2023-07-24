package bybit

import (
	"math"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalMarket(m bybitapi.Instrument) types.Market {
	// sample:
	//Symbol: BTCUSDT
	//BaseCoin: BTC
	//QuoteCoin: USDT
	//Innovation: 0
	//Status: Trading
	//MarginTrading: both
	//
	//LotSizeFilter:
	//{
	//    BasePrecision: 0.000001
	//    QuotePrecision: 0.00000001
	//    MinOrderQty: 0.000048
	//    MaxOrderQty: 71.73956243
	//    MinOrderAmt: 1
	//    MaxOrderAmt: 2000000
	//}
	//
	//PriceFilter:
	//{
	//    TickSize: 0.01
	//}
	return types.Market{
		Symbol:          m.Symbol,
		LocalSymbol:     m.Symbol,
		PricePrecision:  int(math.Log10(m.LotSizeFilter.QuotePrecision.Float64())),
		VolumePrecision: int(math.Log10(m.LotSizeFilter.BasePrecision.Float64())),
		QuoteCurrency:   m.QuoteCoin,
		BaseCurrency:    m.BaseCoin,
		MinNotional:     m.LotSizeFilter.MinOrderAmt,
		MinAmount:       m.LotSizeFilter.MinOrderAmt,

		// quantity
		MinQuantity: m.LotSizeFilter.MinOrderQty,
		MaxQuantity: m.LotSizeFilter.MaxOrderQty,
		StepSize:    m.LotSizeFilter.BasePrecision,

		// price
		MinPrice: m.LotSizeFilter.MinOrderAmt,
		MaxPrice: m.LotSizeFilter.MaxOrderAmt,
		TickSize: m.PriceFilter.TickSize,
	}
}
