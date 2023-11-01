package bitget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_toGlobalBalance(t *testing.T) {
	// sample:
	// {
	//        "coinId":"10012",
	//        "coinName":"usdt",
	//        "available":"0",
	//        "frozen":"0",
	//        "lock":"0",
	//        "uTime":"1622697148"
	//    }
	asset := bitgetapi.AccountAsset{
		CoinId:    2,
		CoinName:  "USDT",
		Available: fixedpoint.NewFromFloat(1.2),
		Frozen:    fixedpoint.NewFromFloat(0.5),
		Lock:      fixedpoint.NewFromFloat(0.5),
		UTime:     types.NewMillisecondTimestampFromInt(1622697148),
	}

	assert.Equal(t, types.Balance{
		Currency:          "USDT",
		Available:         fixedpoint.NewFromFloat(1.2),
		Locked:            fixedpoint.NewFromFloat(1), // frozen + lock
		Borrowed:          fixedpoint.Zero,
		Interest:          fixedpoint.Zero,
		NetAsset:          fixedpoint.Zero,
		MaxWithdrawAmount: fixedpoint.Zero,
	}, toGlobalBalance(asset))
}

func Test_toGlobalMarket(t *testing.T) {
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

func Test_toGlobalTicker(t *testing.T) {
	// sample:
	// {
	//        "symbol": "BTCUSDT",
	//        "high24h": "24175.65",
	//        "low24h": "23677.75",
	//        "close": "24014.11",
	//        "quoteVol": "177689342.3025",
	//        "baseVol": "7421.5009",
	//        "usdtVol": "177689342.302407",
	//        "ts": "1660704288118",
	//        "buyOne": "24013.94",
	//        "sellOne": "24014.06",
	//        "bidSz": "0.0663",
	//        "askSz": "0.0119",
	//        "openUtc0": "23856.72",
	//        "changeUtc":"0.00301",
	//        "change":"0.00069"
	//    }
	ticker := bitgetapi.Ticker{
		Symbol:    "BTCUSDT",
		High24H:   fixedpoint.NewFromFloat(24175.65),
		Low24H:    fixedpoint.NewFromFloat(23677.75),
		Close:     fixedpoint.NewFromFloat(24014.11),
		QuoteVol:  fixedpoint.NewFromFloat(177689342.3025),
		BaseVol:   fixedpoint.NewFromFloat(7421.5009),
		UsdtVol:   fixedpoint.NewFromFloat(177689342.302407),
		Ts:        types.NewMillisecondTimestampFromInt(1660704288118),
		BuyOne:    fixedpoint.NewFromFloat(24013.94),
		SellOne:   fixedpoint.NewFromFloat(24014.06),
		BidSz:     fixedpoint.NewFromFloat(0.0663),
		AskSz:     fixedpoint.NewFromFloat(0.0119),
		OpenUtc0:  fixedpoint.NewFromFloat(23856.72),
		ChangeUtc: fixedpoint.NewFromFloat(0.00301),
		Change:    fixedpoint.NewFromFloat(0.00069),
	}

	assert.Equal(t, types.Ticker{
		Time:   types.NewMillisecondTimestampFromInt(1660704288118).Time(),
		Volume: fixedpoint.NewFromFloat(7421.5009),
		Last:   fixedpoint.NewFromFloat(24014.11),
		Open:   fixedpoint.NewFromFloat(23856.72),
		High:   fixedpoint.NewFromFloat(24175.65),
		Low:    fixedpoint.NewFromFloat(23677.75),
		Buy:    fixedpoint.NewFromFloat(24013.94),
		Sell:   fixedpoint.NewFromFloat(24014.06),
	}, toGlobalTicker(ticker))
}
