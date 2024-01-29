package okex

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Channel string

const (
	ChannelBooks        Channel = "books"
	ChannelBook5        Channel = "book5"
	ChannelCandlePrefix Channel = "candle"
	ChannelAccount      Channel = "account"
	ChannelMarketTrades Channel = "trades"
	ChannelOrderTrades  Channel = "orders"
)

type ActionType string

const (
	ActionTypeSnapshot ActionType = "snapshot"
	ActionTypeUpdate   ActionType = "update"
)

func parseWebSocketEvent(in []byte) (interface{}, error) {
	var event WebSocketEvent
	err := json.Unmarshal(in, &event)
	if err != nil {
		return nil, err
	}
	if event.Event != "" {
		return &event, nil
	}

	switch event.Arg.Channel {
	case ChannelAccount:
		return parseAccount(event.Data)

	case ChannelBooks, ChannelBook5:
		var bookEvent BookEvent
		err = json.Unmarshal(event.Data, &bookEvent.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal data into BookEvent, Arg: %+v Data: %s, err: %w", event.Arg, string(event.Data), err)
		}

		instId := event.Arg.InstId
		bookEvent.InstrumentID = instId
		bookEvent.Symbol = toGlobalSymbol(instId)
		bookEvent.channel = event.Arg.Channel
		bookEvent.Action = event.ActionType
		return &bookEvent, nil

	case ChannelMarketTrades:
		var trade []MarketTradeEvent
		err = json.Unmarshal(event.Data, &trade)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal data into MarketTradeEvent: %+v, err: %w", string(event.Data), err)
		}
		return trade, nil

	case ChannelOrderTrades:
		var orderTrade []OrderTradeEvent
		err := json.Unmarshal(event.Data, &orderTrade)
		if err != nil {
			return nil, err
		}

		return orderTrade, nil

	default:
		if strings.HasPrefix(string(event.Arg.Channel), string(ChannelCandlePrefix)) {
			// TODO: Support kline subscription. The kline requires another URL to subscribe, which is why we cannot
			//  support it at this time.
			var kLineEvt KLineEvent
			err = json.Unmarshal(event.Data, &kLineEvt.Events)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal data into KLineEvent, Arg: %+v Data: %s, err: %w", event.Arg, string(event.Data), err)
			}

			kLineEvt.Channel = event.Arg.Channel
			kLineEvt.InstrumentID = event.Arg.InstId
			kLineEvt.Symbol = toGlobalSymbol(event.Arg.InstId)
			kLineEvt.Interval = strings.ToLower(strings.TrimPrefix(string(event.Arg.Channel), string(ChannelCandlePrefix)))
			return &kLineEvt, nil
		}
	}

	return nil, nil
}

type WsEventType string

const (
	WsEventTypeLogin       = "login"
	WsEventTypeError       = "error"
	WsEventTypeSubscribe   = "subscribe"
	WsEventTypeUnsubscribe = "unsubscribe"
)

type WebSocketEvent struct {
	Event   WsEventType `json:"event"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"msg,omitempty"`
	Arg     struct {
		Channel Channel `json:"channel"`
		InstId  string  `json:"instId"`
	} `json:"arg,omitempty"`
	Data       json.RawMessage `json:"data"`
	ActionType ActionType      `json:"action"`
}

func (w *WebSocketEvent) IsValid() error {
	switch w.Event {
	case WsEventTypeError:
		return fmt.Errorf("websocket request error, code: %s, msg: %s", w.Code, w.Message)

	case WsEventTypeSubscribe, WsEventTypeUnsubscribe:
		return nil

	case WsEventTypeLogin:
		// Actually, this code is unnecessary because the events are either `Subscribe` or `Unsubscribe`, But to avoid bugs
		// in the exchange, we still check.
		if w.Code != "0" || len(w.Message) != 0 {
			return fmt.Errorf("websocket request error, code: %s, msg: %s", w.Code, w.Message)
		}
		return nil

	default:
		return fmt.Errorf("unexpected event type: %+v", w)
	}
}

func (w *WebSocketEvent) IsAuthenticated() bool {
	return w.Event == WsEventTypeLogin && w.Code == "0"
}

type BookEvent struct {
	InstrumentID string
	Symbol       string
	Action       ActionType
	channel      Channel

	Data []struct {
		Bids                 PriceVolumeOrderSlice      `json:"bids"`
		Asks                 PriceVolumeOrderSlice      `json:"asks"`
		MillisecondTimestamp types.MillisecondTimestamp `json:"ts"`
		Checksum             int                        `json:"checksum"`
	}
}

func (event *BookEvent) BookTicker() types.BookTicker {
	ticker := types.BookTicker{
		Symbol: event.Symbol,
	}

	if len(event.Data) > 0 {
		if len(event.Data[0].Bids) > 0 {
			ticker.Buy = event.Data[0].Bids[0].Price
			ticker.BuySize = event.Data[0].Bids[0].Volume
		}

		if len(event.Data[0].Asks) > 0 {
			ticker.Sell = event.Data[0].Asks[0].Price
			ticker.SellSize = event.Data[0].Asks[0].Volume
		}
	}

	return ticker
}

func (event *BookEvent) Book() types.SliceOrderBook {
	book := types.SliceOrderBook{
		Symbol: event.Symbol,
	}

	if len(event.Data) > 0 {
		book.Time = event.Data[0].MillisecondTimestamp.Time()
	}

	for _, data := range event.Data {
		for _, bid := range data.Bids {
			book.Bids = append(book.Bids, types.PriceVolume{Price: bid.Price, Volume: bid.Volume})
		}

		for _, ask := range data.Asks {
			book.Asks = append(book.Asks, types.PriceVolume{Price: ask.Price, Volume: ask.Volume})
		}
	}

	return book
}

type PriceVolumeOrder struct {
	types.PriceVolume
	// NumLiquidated is part of a deprecated feature and it is always "0"
	NumLiquidated int
	// NumOrders is the number of orders at the price.
	NumOrders int
}

type PriceVolumeOrderSlice []PriceVolumeOrder

func (slice *PriceVolumeOrderSlice) UnmarshalJSON(b []byte) error {
	s, err := ParsePriceVolumeOrderSliceJSON(b)
	if err != nil {
		return err
	}

	*slice = s
	return nil
}

// ParsePriceVolumeOrderSliceJSON tries to parse a 2 dimensional string array into a PriceVolumeOrderSlice
//
//	[["8476.98", "415", "0", "13"], ["8477", "7", "0", "2"], ... ]
func ParsePriceVolumeOrderSliceJSON(b []byte) (slice PriceVolumeOrderSlice, err error) {
	var as [][]fixedpoint.Value

	err = json.Unmarshal(b, &as)
	if err != nil {
		return slice, fmt.Errorf("failed to unmarshal price volume order slice: %w", err)
	}

	for _, a := range as {
		var pv PriceVolumeOrder
		pv.Price = a[0]
		pv.Volume = a[1]
		pv.NumLiquidated = a[2].Int()
		pv.NumOrders = a[3].Int()

		slice = append(slice, pv)
	}

	return slice, nil
}

func kLineToGlobal(k okexapi.KLine, interval types.Interval, symbol string) types.KLine {
	startTime := k.StartTime.Time()

	return types.KLine{
		Exchange:                 types.ExchangeOKEx,
		Symbol:                   symbol,
		StartTime:                types.Time(startTime),
		EndTime:                  types.Time(startTime.Add(interval.Duration() - time.Millisecond)),
		Interval:                 interval,
		Open:                     k.OpenPrice,
		Close:                    k.ClosePrice,
		High:                     k.HighestPrice,
		Low:                      k.LowestPrice,
		Volume:                   k.Volume,
		QuoteVolume:              k.VolumeInCurrency, // not supported
		TakerBuyBaseAssetVolume:  fixedpoint.Zero,    // not supported
		TakerBuyQuoteAssetVolume: fixedpoint.Zero,    // not supported
		LastTradeID:              0,                  // not supported
		NumberOfTrades:           0,                  // not supported
		Closed:                   !k.Confirm.IsZero(),
	}
}

type KLineEvent struct {
	Events okexapi.KLineSlice

	InstrumentID string
	Symbol       string
	Interval     string
	Channel      Channel
}

func parseAccount(v []byte) (*okexapi.Account, error) {
	var accounts []okexapi.Account
	err := json.Unmarshal(v, &accounts)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return &okexapi.Account{}, nil
	}

	return &accounts[0], nil
}

type OrderTradeEvent struct {
	okexapi.OrderDetail

	Code          types.StrInt64        `json:"code"`
	Msg           string                `json:"msg"`
	AmendResult   string                `json:"amendResult"`
	ExecutionType okexapi.LiquidityType `json:"execType"`
	// FillFee last filled fee amount or rebate amount:
	// Negative number represents the user transaction fee charged by the platform;
	// Positive number represents rebate
	FillFee fixedpoint.Value `json:"fillFee"`
	// FillFeeCurrency last filled fee currency or rebate currency.
	// It is fee currency when fillFee is less than 0; It is rebate currency when fillFee>=0.
	FillFeeCurrency string `json:"fillFeeCcy"`
	// FillNotionalUsd Filled notional value in USD of order
	FillNotionalUsd fixedpoint.Value `json:"fillNotionalUsd"`
	FillPnl         fixedpoint.Value `json:"fillPnl"`
	// NotionalUsd Estimated national value in USD of order
	NotionalUsd fixedpoint.Value `json:"notionalUsd"`
	// ReqId Client Request ID as assigned by the client for order amendment. "" will be returned if there is no order amendment.
	ReqId     string           `json:"reqId"`
	LastPrice fixedpoint.Value `json:"lastPx"`
	// QuickMgnType Quick Margin type, Only applicable to Quick Margin Mode of isolated margin
	// manual, auto_borrow, auto_repay
	QuickMgnType string `json:"quickMgnType"`
	// AmendSource Source of the order amendation.
	AmendSource string `json:"amendSource"`
	// CancelSource Source of the order cancellation.
	CancelSource string `json:"cancelSource"`

	// Only applicable to options; return "" for other instrument types
	FillPriceVolume string `json:"fillPxVol"`
	FillPriceUsd    string `json:"fillPxUsd"`
	FillMarkVolume  string `json:"fillMarkVol"`
	FillFwdPrice    string `json:"fillFwdPx"`
	FillMarkPrice   string `json:"fillMarkPx"`
}

func (o *OrderTradeEvent) toGlobalTrade() (types.Trade, error) {
	side := toGlobalSide(o.Side)
	tradeId, err := strconv.ParseUint(o.TradeId, 10, 64)
	if err != nil {
		return types.Trade{}, fmt.Errorf("unexpected trade id [%s] format: %w", o.TradeId, err)
	}
	return types.Trade{
		ID:            tradeId,
		OrderID:       uint64(o.OrderId),
		Exchange:      types.ExchangeOKEx,
		Price:         o.FillPrice,
		Quantity:      o.FillSize,
		QuoteQuantity: o.FillPrice.Mul(o.FillSize),
		Symbol:        toGlobalSymbol(o.InstrumentID),
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       o.ExecutionType == okexapi.LiquidityTypeMaker,
		Time:          types.Time(o.FillTime.Time()),
		// charged by the platform is positive in our design, so added the `Neg()`.
		Fee:           o.FillFee.Neg(),
		FeeCurrency:   o.FeeCurrency,
		FeeDiscounted: false,
	}, nil
}

func toGlobalSideType(side okexapi.SideType) (types.SideType, error) {
	switch side {
	case okexapi.SideTypeBuy:
		return types.SideTypeBuy, nil

	case okexapi.SideTypeSell:
		return types.SideTypeSell, nil

	default:
		return types.SideType(side), fmt.Errorf("unexpected side: %s", side)
	}
}

type MarketTradeEvent struct {
	InstId    string                     `json:"instId"`
	TradeId   types.StrInt64             `json:"tradeId"`
	Px        fixedpoint.Value           `json:"px"`
	Sz        fixedpoint.Value           `json:"sz"`
	Side      okexapi.SideType           `json:"side"`
	Timestamp types.MillisecondTimestamp `json:"ts"`
	Count     types.StrInt64             `json:"count"`
}

func (m *MarketTradeEvent) toGlobalTrade() (types.Trade, error) {
	symbol := toGlobalSymbol(m.InstId)
	if symbol == "" {
		return types.Trade{}, fmt.Errorf("unexpected inst id: %s", m.InstId)
	}

	side, err := toGlobalSideType(m.Side)
	if err != nil {
		return types.Trade{}, err
	}

	return types.Trade{
		ID:            uint64(m.TradeId),
		OrderID:       0, // not supported
		Exchange:      types.ExchangeOKEx,
		Price:         m.Px,
		Quantity:      m.Sz,
		QuoteQuantity: m.Px.Mul(m.Sz),
		Symbol:        symbol,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       false, // not supported
		Time:          types.Time(m.Timestamp.Time()),
		Fee:           fixedpoint.Zero, // not supported
		FeeCurrency:   "",              // not supported
	}, nil
}
