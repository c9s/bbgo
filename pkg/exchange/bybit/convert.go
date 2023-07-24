package bybit

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalMarket(m bybitapi.Instrument) types.Market {
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

func toGlobalTicker(stats bybitapi.Ticker, time time.Time) types.Ticker {
	return types.Ticker{
		Volume: stats.Volume24H,
		Last:   stats.LastPrice,
		Open:   stats.PrevPrice24H, // Market price 24 hours ago
		High:   stats.HighPrice24H,
		Low:    stats.LowPrice24H,
		Buy:    stats.Bid1Price,
		Sell:   stats.Ask1Price,
		Time:   time,
	}
}
