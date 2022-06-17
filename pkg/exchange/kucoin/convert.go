package kucoin

import (
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
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
		MinNotional:     m.QuoteMinSize,
		MinAmount:       m.QuoteMinSize,
		MinQuantity:     m.BaseMinSize,
		MaxQuantity:     fixedpoint.Zero, // not used
		StepSize:        m.BaseIncrement,

		MinPrice: fixedpoint.Zero, // not used
		MaxPrice: fixedpoint.Zero, // not used
		TickSize: m.PriceIncrement,
	}
}

func toGlobalTicker(s kucoinapi.Ticker24H) types.Ticker {
	return types.Ticker{
		Time:   s.Time.Time(),
		Volume: s.Volume,
		Last:   s.Last,
		Open:   s.Last.Sub(s.ChangePrice),
		High:   s.High,
		Low:    s.Low,
		Buy:    s.Buy,
		Sell:   s.Sell,
	}
}

func toLocalInterval(i types.Interval) string {
	switch i {
	case types.Interval1m:
		return "1min"

	case types.Interval5m:
		return "5min"

	case types.Interval15m:
		return "15min"

	case types.Interval30m:
		return "30min"

	case types.Interval1h:
		return "1hour"

	case types.Interval2h:
		return "2hour"

	case types.Interval4h:
		return "4hour"

	case types.Interval6h:
		return "6hour"

	case types.Interval12h:
		return "12hour"

	case types.Interval1d:
		return "1day"

	}

	return "1hour"
}

// convertSubscriptions global subscription to local websocket command
func convertSubscriptions(ss []types.Subscription) ([]WebSocketCommand, error) {
	var id = time.Now().UnixNano() / int64(time.Millisecond)
	var cmds []WebSocketCommand
	for _, s := range ss {
		id++

		var subscribeTopic string
		switch s.Channel {
		case types.BookChannel:
			// see https://docs.kucoin.com/#level-2-market-data
			subscribeTopic = "/market/level2" + ":" + toLocalSymbol(s.Symbol)

		case types.KLineChannel:
			subscribeTopic = "/market/candles" + ":" + toLocalSymbol(s.Symbol) + "_" + toLocalInterval(types.Interval(s.Options.Interval))

		default:
			return nil, fmt.Errorf("websocket channel %s is not supported by kucoin", s.Channel)
		}

		cmds = append(cmds, WebSocketCommand{
			Id:             id,
			Type:           WebSocketMessageTypeSubscribe,
			Topic:          subscribeTopic,
			PrivateChannel: false,
			Response:       true,
		})
	}

	return cmds, nil
}

func hashStringID(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func toGlobalOrderStatus(o kucoinapi.Order) types.OrderStatus {
	var status types.OrderStatus
	if o.IsActive {
		status = types.OrderStatusNew
		if o.DealSize.Sign() > 0 {
			status = types.OrderStatusPartiallyFilled
		}
	} else if o.CancelExist {
		status = types.OrderStatusCanceled
	} else {
		status = types.OrderStatusFilled
	}

	return status
}

func toGlobalSide(s string) types.SideType {
	switch s {
	case "buy":
		return types.SideTypeBuy
	case "sell":
		return types.SideTypeSell
	}

	return types.SideTypeSelf
}

func toGlobalOrderType(s string) types.OrderType {
	switch s {
	case "limit":
		return types.OrderTypeLimit

	case "stop_limit":
		return types.OrderTypeStopLimit

	case "market":
		return types.OrderTypeMarket

	case "stop_market":
		return types.OrderTypeStopMarket

	}

	return ""
}

func toLocalSide(side types.SideType) kucoinapi.SideType {
	switch side {
	case types.SideTypeBuy:
		return kucoinapi.SideTypeBuy

	case types.SideTypeSell:
		return kucoinapi.SideTypeSell

	}

	return ""
}

func toGlobalOrder(o kucoinapi.Order) types.Order {
	var status = toGlobalOrderStatus(o)
	var order = types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: o.ClientOrderID,
			Symbol:        toGlobalSymbol(o.Symbol),
			Side:          toGlobalSide(o.Side),
			Type:          toGlobalOrderType(o.Type),
			Quantity:      o.Size,
			Price:         o.Price,
			StopPrice:     o.StopPrice,
			TimeInForce:   types.TimeInForce(o.TimeInForce),
		},
		Exchange:         types.ExchangeKucoin,
		OrderID:          hashStringID(o.ID),
		UUID:             o.ID,
		Status:           status,
		ExecutedQuantity: o.DealSize,
		IsWorking:        o.IsActive,
		CreationTime:     types.Time(o.CreatedAt.Time()),
		UpdateTime:       types.Time(o.CreatedAt.Time()), // kucoin does not response updated time
	}
	return order
}

func toGlobalTrade(fill kucoinapi.Fill) types.Trade {
	var trade = types.Trade{
		ID:            hashStringID(fill.TradeId),
		OrderID:       hashStringID(fill.OrderId),
		Exchange:      types.ExchangeKucoin,
		Price:         fill.Price,
		Quantity:      fill.Size,
		QuoteQuantity: fill.Funds,
		Symbol:        toGlobalSymbol(fill.Symbol),
		Side:          toGlobalSide(string(fill.Side)),
		IsBuyer:       fill.Side == kucoinapi.SideTypeBuy,
		IsMaker:       fill.Liquidity == kucoinapi.LiquidityTypeMaker,
		Time:          types.Time(fill.CreatedAt.Time()),
		Fee:           fill.Fee,
		FeeCurrency:   toGlobalSymbol(fill.FeeCurrency),
	}
	return trade
}
