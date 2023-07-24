package bybit

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestToGlobalMarket(t *testing.T) {
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
