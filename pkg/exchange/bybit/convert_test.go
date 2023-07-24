package bybit

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestToGlobalMarket(t *testing.T) {
	// sample:
	//{
	//	"Symbol": "BTCUSDT",
	//	"BaseCoin": "BTC",
	//	"QuoteCoin": "USDT",
	//	"Innovation": 0,
	//	"Status": "Trading",
	//	"MarginTrading": "both",
	//	"LotSizeFilter": {
	//	"BasePrecision": 0.000001,
	//		"QuotePrecision": 0.00000001,
	//		"MinOrderQty": 0.000048,
	//		"MaxOrderQty": 71.73956243,
	//		"MinOrderAmt": 1,
	//		"MaxOrderAmt": 2000000
	//	},
	//	"PriceFilter": {
	//		"TickSize": 0.01
	//	}
	//}
	inst := bybitapi.Instrument{
		Symbol:        "BTCUSDT",
		BaseCoin:      "BTC",
		QuoteCoin:     "USDT",
		Innovation:    "0",
		Status:        bybitapi.StatusTrading,
		MarginTrading: "both",
		LotSizeFilter: struct {
			BasePrecision  fixedpoint.Value `json:"basePrecision"`
			QuotePrecision fixedpoint.Value `json:"quotePrecision"`
			MinOrderQty    fixedpoint.Value `json:"minOrderQty"`
			MaxOrderQty    fixedpoint.Value `json:"maxOrderQty"`
			MinOrderAmt    fixedpoint.Value `json:"minOrderAmt"`
			MaxOrderAmt    fixedpoint.Value `json:"maxOrderAmt"`
		}{
			BasePrecision:  fixedpoint.NewFromFloat(0.000001),
			QuotePrecision: fixedpoint.NewFromFloat(0.00000001),
			MinOrderQty:    fixedpoint.NewFromFloat(0.000048),
			MaxOrderQty:    fixedpoint.NewFromFloat(71.73956243),
			MinOrderAmt:    fixedpoint.NewFromInt(1),
			MaxOrderAmt:    fixedpoint.NewFromInt(2000000),
		},
		PriceFilter: struct {
			TickSize fixedpoint.Value `json:"tickSize"`
		}{
			TickSize: fixedpoint.NewFromFloat(0.01),
		},
	}

	exp := types.Market{
		Symbol:          inst.Symbol,
		LocalSymbol:     inst.Symbol,
		PricePrecision:  int(math.Log10(inst.LotSizeFilter.QuotePrecision.Float64())),
		VolumePrecision: int(math.Log10(inst.LotSizeFilter.BasePrecision.Float64())),
		QuoteCurrency:   inst.QuoteCoin,
		BaseCurrency:    inst.BaseCoin,
		MinNotional:     inst.LotSizeFilter.MinOrderAmt,
		MinAmount:       inst.LotSizeFilter.MinOrderAmt,
		MinQuantity:     inst.LotSizeFilter.MinOrderQty,
		MaxQuantity:     inst.LotSizeFilter.MaxOrderQty,
		StepSize:        inst.LotSizeFilter.BasePrecision,
		MinPrice:        inst.LotSizeFilter.MinOrderAmt,
		MaxPrice:        inst.LotSizeFilter.MaxOrderAmt,
		TickSize:        inst.PriceFilter.TickSize,
	}

	assert.Equal(t, toGlobalMarket(inst), exp)
}

func TestToGlobalTicker(t *testing.T) {
	// sample
	//{
	// 	  "symbol": "BTCUSDT",
	//    "bid1Price": "28995.98",
	//    "bid1Size": "4.741552",
	//    "ask1Price": "28995.99",
	//    "ask1Size": "0.16075",
	//    "lastPrice": "28994",
	//    "prevPrice24h": "29900",
	//    "price24hPcnt": "-0.0303",
	//    "highPrice24h": "30344.78",
	//    "lowPrice24h": "28948.87",
	//    "turnover24h": "184705500.13172874",
	//    "volume24h": "6240.807096",
	//    "usdIndexPrice": "28977.82001643"
	//}
	ticker := bybitapi.Ticker{
		Symbol:        "BTCUSDT",
		Bid1Price:     fixedpoint.NewFromFloat(28995.98),
		Bid1Size:      fixedpoint.NewFromFloat(4.741552),
		Ask1Price:     fixedpoint.NewFromFloat(28995.99),
		Ask1Size:      fixedpoint.NewFromFloat(0.16075),
		LastPrice:     fixedpoint.NewFromFloat(28994),
		PrevPrice24H:  fixedpoint.NewFromFloat(29900),
		Price24HPcnt:  fixedpoint.NewFromFloat(-0.0303),
		HighPrice24H:  fixedpoint.NewFromFloat(30344.78),
		LowPrice24H:   fixedpoint.NewFromFloat(28948.87),
		Turnover24H:   fixedpoint.NewFromFloat(184705500.13172874),
		Volume24H:     fixedpoint.NewFromFloat(6240.807096),
		UsdIndexPrice: fixedpoint.NewFromFloat(28977.82001643),
	}

	timeNow := time.Now()

	exp := types.Ticker{
		Time:   timeNow,
		Volume: ticker.Volume24H,
		Last:   ticker.LastPrice,
		Open:   ticker.PrevPrice24H,
		High:   ticker.HighPrice24H,
		Low:    ticker.LowPrice24H,
		Buy:    ticker.Bid1Price,
		Sell:   ticker.Ask1Price,
	}

	assert.Equal(t, toGlobalTicker(ticker, timeNow), exp)
}
