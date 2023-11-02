package bitget

import (
	"math"
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalSymbol(s string) string {
	return strings.ToUpper(s)
}

func toGlobalBalance(asset bitgetapi.AccountAsset) types.Balance {
	return types.Balance{
		Currency:          asset.CoinName,
		Available:         asset.Available,
		Locked:            asset.Lock.Add(asset.Frozen),
		Borrowed:          fixedpoint.Zero,
		Interest:          fixedpoint.Zero,
		NetAsset:          fixedpoint.Zero,
		MaxWithdrawAmount: fixedpoint.Zero,
	}
}

func toGlobalMarket(s bitgetapi.Symbol) types.Market {
	if s.Status != bitgetapi.SymbolOnline {
		log.Warnf("The symbol %s is not online", s.Symbol)
	}
	return types.Market{
		Symbol:          s.SymbolName,
		LocalSymbol:     s.Symbol,
		PricePrecision:  s.PriceScale.Int(),
		VolumePrecision: s.QuantityScale.Int(),
		QuoteCurrency:   s.QuoteCoin,
		BaseCurrency:    s.BaseCoin,
		MinNotional:     s.MinTradeUSDT,
		MinAmount:       s.MinTradeUSDT,
		MinQuantity:     s.MinTradeAmount,
		MaxQuantity:     s.MaxTradeAmount,
		StepSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(s.QuantityScale.Int())),
		TickSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(s.PriceScale.Int())),
		MinPrice:        fixedpoint.Zero,
		MaxPrice:        fixedpoint.Zero,
	}
}

func toGlobalTicker(ticker bitgetapi.Ticker) types.Ticker {
	return types.Ticker{
		Time:   ticker.Ts.Time(),
		Volume: ticker.BaseVol,
		Last:   ticker.Close,
		Open:   ticker.OpenUtc0,
		High:   ticker.High24H,
		Low:    ticker.Low24H,
		Buy:    ticker.BuyOne,
		Sell:   ticker.SellOne,
	}
}
