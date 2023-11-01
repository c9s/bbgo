package bitget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestToGlobalMarket(t *testing.T) {
	// sample:
	//{
	//            "symbol":"BTCUSDT_SPBL",
	//            "symbolName":"BTCUSDT",
	//            "baseCoin":"BTC",
	//            "quoteCoin":"USDT",
	//            "minTradeAmount":"0.0001",
	//            "maxTradeAmount":"10000",
	//            "takerFeeRate":"0.001",
	//            "makerFeeRate":"0.001",
	//            "priceScale":"4",
	//            "quantityScale":"8",
	//            "minTradeUSDT":"5",
	//            "status":"online",
	//            "buyLimitPriceRatio": "0.05",
	//            "sellLimitPriceRatio": "0.05"
	//        }
	inst := bitgetapi.Symbol{
		Symbol:              "BTCUSDT_SPBL",
		SymbolName:          "BTCUSDT",
		BaseCoin:            "BTC",
		QuoteCoin:           "USDT",
		MinTradeAmount:      fixedpoint.NewFromFloat(0.0001),
		MaxTradeAmount:      fixedpoint.NewFromFloat(10000),
		TakerFeeRate:        fixedpoint.NewFromFloat(0.001),
		MakerFeeRate:        fixedpoint.NewFromFloat(0.001),
		PriceScale:          fixedpoint.NewFromFloat(4),
		QuantityScale:       fixedpoint.NewFromFloat(8),
		MinTradeUSDT:        fixedpoint.NewFromFloat(5),
		Status:              bitgetapi.SymbolOnline,
		BuyLimitPriceRatio:  fixedpoint.NewFromFloat(0.05),
		SellLimitPriceRatio: fixedpoint.NewFromFloat(0.05),
	}

	exp := types.Market{
		Symbol:          inst.SymbolName,
		LocalSymbol:     inst.Symbol,
		PricePrecision:  4,
		VolumePrecision: 8,
		QuoteCurrency:   inst.QuoteCoin,
		BaseCurrency:    inst.BaseCoin,
		MinNotional:     inst.MinTradeUSDT,
		MinAmount:       inst.MinTradeUSDT,
		MinQuantity:     inst.MinTradeAmount,
		MaxQuantity:     inst.MaxTradeAmount,
		StepSize:        fixedpoint.NewFromFloat(0.00000001),
		MinPrice:        fixedpoint.Zero,
		MaxPrice:        fixedpoint.Zero,
		TickSize:        fixedpoint.NewFromFloat(0.0001),
	}

	assert.Equal(t, toGlobalMarket(inst), exp)
}
