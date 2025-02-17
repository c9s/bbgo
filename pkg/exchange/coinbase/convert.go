package coinbase

import (
	"hash/fnv"
	"math"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalOrder(cbOrder *api.Order) *types.Order {
	return &types.Order{
		Exchange:       types.ExchangeCoinBase,
		Status:         cbOrder.Status.GlobalOrderStatus(),
		UUID:           cbOrder.ID,
		OrderID:        FNV64a(cbOrder.ID),
		OriginalStatus: string(cbOrder.Status),
		CreationTime:   types.Time(cbOrder.CreatedAt),
	}
}

func toGlobalTrade(cbTrade *api.Trade) *types.Trade {
	return &types.Trade{
		ID:            uint64(cbTrade.TradeID),
		OrderID:       FNV64a(cbTrade.OrderID),
		Exchange:      types.ExchangeCoinBase,
		Price:         cbTrade.Price,
		Quantity:      cbTrade.Size,
		QuoteQuantity: cbTrade.Size.Mul(cbTrade.Price),
		Symbol:        cbTrade.ProductID,
		Side:          cbTrade.Side.GlobalSideType(),
		IsBuyer:       cbTrade.Liquidity == api.LiquidityTaker,
		IsMaker:       cbTrade.Liquidity == api.LiquidityMaker,
		Fee:           cbTrade.Fee,
		FeeCurrency:   cbTrade.FundingCurrency,
	}
}

func toGlobalMarket(cbMarket *api.MarketInfo) *types.Market {
	pricePrecision := int(math.Log10(fixedpoint.One.Div(cbMarket.QuoteIncrement).Float64()))
	volumnPrecision := int(math.Log10(fixedpoint.One.Div(cbMarket.BaseIncrement).Float64()))
	return &types.Market{
		Exchange:        types.ExchangeCoinBase,
		Symbol:          toGlobalSymbol(cbMarket.ID),
		LocalSymbol:     cbMarket.ID,
		PricePrecision:  pricePrecision,
		VolumePrecision: volumnPrecision,
		QuoteCurrency:   cbMarket.QuoteCurrency,
		BaseCurrency:    cbMarket.BaseCurrency,
		MinNotional:     cbMarket.MinMarketFunds,
	}
}

func FNV64a(text string) uint64 {
	hash := fnv.New64a()
	// In hash implementation, it says never return an error.
	_, _ = hash.Write([]byte(text))
	return hash.Sum64()
}

func toGlobalKline(symbol string, granity string, candle *api.Candle) *types.KLine {
	kline := types.KLine{
		Exchange:  types.ExchangeCoinBase,
		Symbol:    symbol,
		StartTime: types.Time(candle.Time),
		EndTime:   types.Time(candle.Time.Add(types.Interval(granity).Duration())),
		Interval:  types.Interval(granity),
		Open:      candle.Open,
		Close:     candle.Close,
		High:      candle.High,
		Low:       candle.Low,
		Volume:    candle.Volume,
	}
	return &kline
}

func toGlobalTicker(cbTicker *api.Ticker) *types.Ticker {
	ticker := types.Ticker{
		Time:   cbTicker.Time,
		Volume: cbTicker.Volume,
		Buy:    cbTicker.Bid,
		Sell:   cbTicker.Ask,
	}
	return &ticker
}

func toGlobalBalance(cur string, cbBalance *api.Balance) *types.Balance {
	balance := types.NewZeroBalance(cur)
	balance.Available = cbBalance.Available
	balance.Locked = cbBalance.Balance.Sub(cbBalance.Available)
	balance.NetAsset = cbBalance.Balance
	return &balance
}
