package bitget

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type InstType string

const (
	instSp InstType = "sp"
)

type ChannelType string

const (
	// ChannelOrderBook snapshot and update might return less than 200 bids/asks as per symbol's orderbook various from
	// each other; The number of bids/asks is not a fixed value and may vary in the future
	ChannelOrderBook ChannelType = "books"
	// ChannelOrderBook5 top 5 order book of "books" that begins from bid1/ask1
	ChannelOrderBook5 ChannelType = "books5"
	// ChannelOrderBook15 top 15 order book of "books" that begins from bid1/ask1
	ChannelOrderBook15 ChannelType = "books15"
	ChannelTrade       ChannelType = "trade"
)

type WsArg struct {
	InstType InstType    `json:"instType"`
	Channel  ChannelType `json:"channel"`
	// InstId Instrument ID. e.q. BTCUSDT, ETHUSDT
	InstId string `json:"instId"`
}

type WsEventType string

const (
	WsEventSubscribe   WsEventType = "subscribe"
	WsEventUnsubscribe WsEventType = "unsubscribe"
	WsEventError       WsEventType = "error"
)

type WsOp struct {
	Op   WsEventType `json:"op"`
	Args []WsArg     `json:"args"`
}

// WsEvent is the lowest level of event type. We use this struct to convert the received data, so that we will know
// whether the event belongs to `op` or `data`.
type WsEvent struct {
	// for comment event
	Arg WsArg `json:"arg"`

	// for op event
	Event WsEventType `json:"event"`
	Code  int         `json:"code"`
	Msg   string      `json:"msg"`
	Op    string      `json:"op"`

	// for data event
	Action ActionType      `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// IsOp represents the data event will be empty
func (w *WsEvent) IsOp() bool {
	return w.Action == "" && len(w.Data) == 0
}

func (w *WsEvent) IsValid() error {
	switch w.Event {
	case WsEventError:
		return fmt.Errorf("websocket request error, op: %s, code: %d, msg: %s", w.Op, w.Code, w.Msg)

	case WsEventSubscribe, WsEventUnsubscribe:
		// Actually, this code is unnecessary because the events are either `Subscribe` or `Unsubscribe`, But to avoid bugs
		// in the exchange, we still check.
		if w.Code != 0 || len(w.Msg) != 0 {
			return fmt.Errorf("unexpected websocket %s event, code: %d, msg: %s", w.Event, w.Code, w.Msg)
		}
		return nil

	default:
		return fmt.Errorf("unexpected event type: %+v", w)
	}
}

type ActionType string

const (
	ActionTypeSnapshot ActionType = "snapshot"
	ActionTypeUpdate   ActionType = "update"
)

//	{
//	   "asks":[
//	      [
//	         "28350.78",
//	         "0.2082"
//	      ],
//	   ],
//	   "bids":[
//	      [
//	         "28350.70",
//	         "0.5585"
//	      ],
//	   ],
//	   "checksum":0,
//	   "ts":"1697593934630"
//	}
type BookEvent struct {
	Events []struct {
		// Order book on sell side, ascending order
		Asks types.PriceVolumeSlice `json:"asks"`
		// Order book on buy side, descending order
		Bids     types.PriceVolumeSlice     `json:"bids"`
		Ts       types.MillisecondTimestamp `json:"ts"`
		Checksum int                        `json:"checksum"`
	}

	// internal use
	actionType ActionType
	instId     string
}

func (e *BookEvent) ToGlobalOrderBooks() []types.SliceOrderBook {
	books := make([]types.SliceOrderBook, len(e.Events))
	for i, event := range e.Events {
		books[i] = types.SliceOrderBook{
			Symbol: e.instId,
			Bids:   event.Bids,
			Asks:   event.Asks,
			Time:   event.Ts.Time(),
		}
	}

	return books
}

type SideType string

const (
	SideBuy  SideType = "buy"
	SideSell SideType = "sell"
)

func (s SideType) ToGlobal() (types.SideType, error) {
	switch s {
	case SideBuy:
		return types.SideTypeBuy, nil
	case SideSell:
		return types.SideTypeSell, nil
	default:
		return "", fmt.Errorf("unexpceted side type: %s", s)
	}
}

type MarketTrade struct {
	Ts    types.MillisecondTimestamp
	Price fixedpoint.Value
	Size  fixedpoint.Value
	Side  SideType
}

type MarketTradeSlice []MarketTrade

func (m *MarketTradeSlice) UnmarshalJSON(b []byte) error {
	if m == nil {
		return errors.New("nil pointer of market trade slice")
	}
	s, err := parseMarketTradeSliceJSON(b)
	if err != nil {
		return err
	}

	*m = s
	return nil
}

// ParseMarketTradeSliceJSON tries to parse a 2 dimensional string array into a MarketTradeSlice
//
// [
//
//	[
//	   "1697694819663",
//	   "28312.97",
//	   "0.1653",
//	   "sell"
//	],
//	[
//	   "1697694818663",
//	   "28313",
//	   "0.1598",
//	   "buy"
//	]
//
// ]
func parseMarketTradeSliceJSON(in []byte) (slice MarketTradeSlice, err error) {
	var rawTrades [][]json.RawMessage

	err = json.Unmarshal(in, &rawTrades)
	if err != nil {
		return slice, err
	}

	for _, raw := range rawTrades {
		if len(raw) != 4 {
			return nil, fmt.Errorf("unexpected trades length: %d, data: %q", len(raw), raw)
		}
		var trade MarketTrade
		if err = json.Unmarshal(raw[0], &trade.Ts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into timestamp: %q", raw[0])
		}
		if err = json.Unmarshal(raw[1], &trade.Price); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into price: %q", raw[1])
		}
		if err = json.Unmarshal(raw[2], &trade.Size); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into size: %q", raw[2])
		}
		if err = json.Unmarshal(raw[3], &trade.Side); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into side: %q", raw[3])
		}

		slice = append(slice, trade)
	}

	return slice, nil
}

func (m MarketTrade) ToGlobal(symbol string) (types.Trade, error) {
	side, err := m.Side.ToGlobal()
	if err != nil {
		return types.Trade{}, err
	}

	return types.Trade{
		ID:            0, // not supported
		OrderID:       0, // not supported
		Exchange:      types.ExchangeBitget,
		Price:         m.Price,
		Quantity:      m.Size,
		QuoteQuantity: m.Price.Mul(m.Size),
		Symbol:        symbol,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       false, // not supported
		Time:          types.Time(m.Ts.Time()),
		Fee:           fixedpoint.Zero, // not supported
		FeeCurrency:   "",              // not supported
	}, nil
}

type MarketTradeEvent struct {
	Events MarketTradeSlice

	// internal use
	actionType ActionType
	instId     string
}

var (
	toLocalInterval = map[types.Interval]string{
		types.Interval1m:  "candle1m",
		types.Interval5m:  "candle5m",
		types.Interval15m: "candle15m",
		types.Interval30m: "candle30m",
		types.Interval1h:  "candle1H",
		types.Interval4h:  "candle4H",
		types.Interval12h: "candle12H",
		types.Interval1d:  "candle1D",
		types.Interval1w:  "candle1W",
	}

	toGlobalInterval = map[string]types.Interval{
		"candle1m":  types.Interval1m,
		"candle5m":  types.Interval5m,
		"candle15m": types.Interval15m,
		"candle30m": types.Interval30m,
		"candle1H":  types.Interval1h,
		"candle4H":  types.Interval4h,
		"candle12H": types.Interval12h,
		"candle1D":  types.Interval1d,
		"candle1W":  types.Interval1w,
	}
)

type KLine struct {
	StartTime    types.MillisecondTimestamp
	OpenPrice    fixedpoint.Value
	HighestPrice fixedpoint.Value
	LowestPrice  fixedpoint.Value
	ClosePrice   fixedpoint.Value
	Volume       fixedpoint.Value
}

func (k KLine) ToGlobal(interval types.Interval, symbol string) types.KLine {
	startTime := k.StartTime.Time()

	return types.KLine{
		Exchange:                 types.ExchangeBitget,
		Symbol:                   symbol,
		StartTime:                types.Time(startTime),
		EndTime:                  types.Time(startTime.Add(interval.Duration() - time.Millisecond)),
		Interval:                 interval,
		Open:                     k.OpenPrice,
		Close:                    k.ClosePrice,
		High:                     k.HighestPrice,
		Low:                      k.LowestPrice,
		Volume:                   k.Volume,
		QuoteVolume:              fixedpoint.Zero, // not supported
		TakerBuyBaseAssetVolume:  fixedpoint.Zero, // not supported
		TakerBuyQuoteAssetVolume: fixedpoint.Zero, // not supported
		LastTradeID:              0,               // not supported
		NumberOfTrades:           0,               // not supported
		Closed:                   false,
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
//	[
//
//	    ["1597026383085", "8533.02", "8553.74", "8527.17", "8548.26", "45247"]
//	]
func parseKLineSliceJSON(in []byte) (slice KLineSlice, err error) {
	var rawKLines [][]json.RawMessage

	err = json.Unmarshal(in, &rawKLines)
	if err != nil {
		return slice, err
	}

	for _, raw := range rawKLines {
		if len(raw) != 6 {
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

		slice = append(slice, kline)
	}

	return slice, nil
}

type KLineEvent struct {
	Events KLineSlice

	// internal use
	actionType ActionType
	channel    ChannelType
	instId     string
}

func (k KLineEvent) CacheKey() string {
	// e.q: candle5m.BTCUSDT
	return fmt.Sprintf("%s.%s", k.channel, k.instId)
}
