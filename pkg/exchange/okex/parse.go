package okex

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/valyala/fastjson"

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
	ChannelOrders       Channel = "orders"
)

type ActionType string

const (
	ActionTypeSnapshot ActionType = "snapshot"
	ActionTypeUpdate   ActionType = "update"
)

func parseWebSocketEvent(in []byte) (interface{}, error) {
	v, err := fastjson.ParseBytes(in)
	if err != nil {
		return nil, err
	}

	var event WebSocketEvent
	err = json.Unmarshal(in, &event)
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

	case ChannelOrders:
		// TODO: remove fastjson
		return parseOrder(v)

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

type KLine struct {
	StartTime    types.MillisecondTimestamp
	OpenPrice    fixedpoint.Value
	HighestPrice fixedpoint.Value
	LowestPrice  fixedpoint.Value
	ClosePrice   fixedpoint.Value
	// Volume trading volume, with a unit of contract.cccccbcvefkeibbhtrebbfklrbetukhrgjgkiilufbde

	// If it is a derivatives contract, the value is the number of contracts.
	// If it is SPOT/MARGIN, the value is the quantity in base currency.
	Volume fixedpoint.Value
	// VolumeCcy trading volume, with a unit of currency.
	// If it is a derivatives contract, the value is the number of base currency.
	// If it is SPOT/MARGIN, the value is the quantity in quote currency.
	VolumeCcy fixedpoint.Value
	// VolumeCcyQuote Trading volume, the value is the quantity in quote currency
	// e.g. The unit is USDT for BTC-USDT and BTC-USDT-SWAP;
	// The unit is USD for BTC-USD-SWAP
	VolumeCcyQuote fixedpoint.Value
	// The state of candlesticks.
	// 0 represents that it is uncompleted, 1 represents that it is completed.
	Confirm fixedpoint.Value
}

func (k KLine) ToGlobal(interval types.Interval, symbol string) types.KLine {
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
		QuoteVolume:              k.VolumeCcy,     // not supported
		TakerBuyBaseAssetVolume:  fixedpoint.Zero, // not supported
		TakerBuyQuoteAssetVolume: fixedpoint.Zero, // not supported
		LastTradeID:              0,               // not supported
		NumberOfTrades:           0,               // not supported
		Closed:                   !k.Confirm.IsZero(),
	}
}

type KLineSlice []KLine

func (m *KLineSlice) UnmarshalJSON(b []byte) error {
	if m == nil {
		return errors.New("nil pointer of kline slice")
	}
	s, err := parseKLineSliceJSON(b)
	if err != nil {
		return err
	}

	*m = s
	return nil
}

// parseKLineSliceJSON tries to parse a 2 dimensional string array into a KLineSlice
//
//		[
//	   [
//	     "1597026383085",
//	     "8533.02",
//	     "8553.74",
//	     "8527.17",
//	     "8548.26",
//	     "45247",
//	     "529.5858061",
//	     "5529.5858061",
//	     "0"
//	   ]
//	 ]
func parseKLineSliceJSON(in []byte) (slice KLineSlice, err error) {
	var rawKLines [][]json.RawMessage

	err = json.Unmarshal(in, &rawKLines)
	if err != nil {
		return slice, err
	}

	for _, raw := range rawKLines {
		if len(raw) != 9 {
			return nil, fmt.Errorf("unexpected kline length: %d, data: %q", len(raw), raw)
		}
		var kline KLine
		if err = json.Unmarshal(raw[0], &kline.StartTime); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into timestamp: %q", raw[0])
		}
		if err = json.Unmarshal(raw[1], &kline.OpenPrice); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into open price: %q", raw[1])
		}
		if err = json.Unmarshal(raw[2], &kline.HighestPrice); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into highest price: %q", raw[2])
		}
		if err = json.Unmarshal(raw[3], &kline.LowestPrice); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into lowest price: %q", raw[3])
		}
		if err = json.Unmarshal(raw[4], &kline.ClosePrice); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into close price: %q", raw[4])
		}
		if err = json.Unmarshal(raw[5], &kline.Volume); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into volume: %q", raw[5])
		}
		if err = json.Unmarshal(raw[6], &kline.VolumeCcy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into volume currency: %q", raw[6])
		}
		if err = json.Unmarshal(raw[7], &kline.VolumeCcyQuote); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into trading currency quote: %q", raw[7])
		}
		if err = json.Unmarshal(raw[8], &kline.Confirm); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into confirm: %q", raw[8])
		}

		slice = append(slice, kline)
	}

	return slice, nil
}

type KLineEvent struct {
	Events KLineSlice

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

func parseOrder(v *fastjson.Value) ([]okexapi.OrderDetails, error) {
	data := v.Get("data").MarshalTo(nil)

	var orderDetails []okexapi.OrderDetails
	err := json.Unmarshal(data, &orderDetails)
	if err != nil {
		return nil, err
	}

	return orderDetails, nil
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
