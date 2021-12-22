package kucoin

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalBalanceMap(accounts []kucoinapi.Account) types.BalanceMap {
	balances := types.BalanceMap{}

	// for now, we only return the trading account
	for _, account := range accounts {
		switch account.Type {
		case kucoinapi.AccountTypeTrade:
			balances[account.Currency] = types.Balance{
				Currency:  account.Currency,
				Available: account.Available,
				Locked:    account.Holds,
			}
		}
	}

	return balances
}

func toGlobalSymbol(symbol string) string {
	return strings.ReplaceAll(symbol, "-", "")
}

func toGlobalMarket(m kucoinapi.Symbol) types.Market {
	symbol := toGlobalSymbol(m.Symbol)
	return types.Market{
		Symbol:          symbol,
		LocalSymbol:     m.Symbol,
		PricePrecision:  int(math.Log10(m.PriceIncrement.Float64())), // convert 0.0001 to 4
		VolumePrecision: int(math.Log10(m.BaseIncrement.Float64())),
		QuoteCurrency:   m.QuoteCurrency,
		BaseCurrency:    m.BaseCurrency,
		MinNotional:     m.QuoteMinSize.Float64(),
		MinAmount:       m.QuoteMinSize.Float64(),
		MinQuantity:     m.BaseMinSize.Float64(),
		MaxQuantity:     0, // not used
		StepSize:        m.BaseIncrement.Float64(),

		MinPrice: 0, // not used
		MaxPrice: 0, // not used
		TickSize: m.PriceIncrement.Float64(),
	}
}

func toGlobalTicker(s kucoinapi.Ticker24H) types.Ticker {
	return types.Ticker{
		Time:   s.Time.Time(),
		Volume: s.Volume.Float64(),
		Last:   s.Last.Float64(),
		Open:   s.Last.Float64() - s.ChangePrice.Float64(),
		High:   s.High.Float64(),
		Low:    s.Low.Float64(),
		Buy:    s.Buy.Float64(),
		Sell:   s.Sell.Float64(),
	}
}


// convertSubscriptions global subscription to local websocket command
func convertSubscriptions(ss []types.Subscription) ([]kucoinapi.WebSocketCommand, error) {
	var id = time.Now().UnixMilli()
	var cmds []kucoinapi.WebSocketCommand
	for _, s := range ss {
		id++

		var subscribeTopic string
		switch s.Channel {
		case types.BookChannel:
			// see https://docs.kucoin.com/#level-2-market-data
			subscribeTopic = "/market/level2" + ":" + toLocalSymbol(s.Symbol)

		case types.KLineChannel:
			subscribeTopic = "/market/candles" + ":" + toLocalSymbol(s.Symbol) + "_" + s.Options.Interval

		default:
			return nil, fmt.Errorf("websocket channel %s is not supported by kucoin", s.Channel)
		}

		cmds = append(cmds, kucoinapi.WebSocketCommand{
			Id:             id,
			Type:           kucoinapi.WebSocketMessageTypeSubscribe,
			Topic:          subscribeTopic,
			PrivateChannel: false,
			Response:       true,
		})
	}

	return cmds, nil
}

