package bitget

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	v2 "github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi/v2"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/strint"
)

type InstType string

const (
	instSpV2 InstType = "SPOT"
)

type ChannelType string

const (
	ChannelAccount ChannelType = "account"
	// ChannelOrderBook snapshot and update might return less than 200 bids/asks as per symbol's orderbook various from
	// each other; The number of bids/asks is not a fixed value and may vary in the future
	ChannelOrderBook ChannelType = "books"
	// ChannelOrderBook5 top 5 order book of "books" that begins from bid1/ask1
	ChannelOrderBook5 ChannelType = "books5"
	// ChannelOrderBook15 top 15 order book of "books" that begins from bid1/ask1
	ChannelOrderBook15 ChannelType = "books15"
	ChannelTrade       ChannelType = "trade"
	ChannelOrders      ChannelType = "orders"
)

type WsArg struct {
	InstType InstType    `json:"instType"`
	Channel  ChannelType `json:"channel"`
	// InstId Instrument ID. e.q. BTCUSDT, ETHUSDT
	InstId string `json:"instId"`
	Coin   string `json:"coin"`

	ApiKey     string `json:"apiKey"`
	Passphrase string `json:"passphrase"`
	Timestamp  string `json:"timestamp"`
	Sign       string `json:"sign"`
}

type WsEventType string

const (
	WsEventSubscribe   WsEventType = "subscribe"
	WsEventUnsubscribe WsEventType = "unsubscribe"
	WsEventLogin       WsEventType = "login"
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

	case WsEventSubscribe, WsEventUnsubscribe, WsEventLogin:
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

func (w *WsEvent) IsAuthenticated() bool {
	return w.Event == WsEventLogin && w.Code == 0
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

func (s *SideType) UnmarshalJSON(b []byte) error {
	var a string
	err := json.Unmarshal(b, &a)
	if err != nil {
		return err
	}

	switch SideType(a) {
	case SideSell, SideBuy:
		*s = SideType(a)
		return nil
	default:
		return fmt.Errorf("unexpected side type: %s", b)
	}
}

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
	Ts      types.MillisecondTimestamp
	Price   fixedpoint.Value
	Size    fixedpoint.Value
	Side    SideType
	TradeId strint.Int64
}

type MarketTradeSlice []MarketTrade

func (m MarketTrade) ToGlobal(symbol string) (types.Trade, error) {
	side, err := m.Side.ToGlobal()
	if err != nil {
		return types.Trade{}, err
	}

	return types.Trade{
		ID:            uint64(m.TradeId),
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
		types.Interval6h:  "candle6Hutc",
		types.Interval12h: "candle12Hutc",
		types.Interval1d:  "candle1Dutc",
		types.Interval3d:  "candle3Dutc",
		types.Interval1w:  "candle1Wutc",
		types.Interval1mo: "candle1Mutc",
	}

	toGlobalInterval = map[string]types.Interval{
		"candle1m":     types.Interval1m,
		"candle5m":     types.Interval5m,
		"candle15m":    types.Interval15m,
		"candle30m":    types.Interval30m,
		"candle1H":     types.Interval1h,
		"candle4H":     types.Interval4h,
		"candle6Hutc":  types.Interval6h,
		"candle12Hutc": types.Interval12h,
		"candle1Dutc":  types.Interval1d,
		"candle3Dutc":  types.Interval3d,
		"candle1Wutc":  types.Interval1w,
		"candle1Mutc":  types.Interval1mo,
	}

	// we align utc time zone
	toLocalGranularity = map[types.Interval]string{
		types.Interval1m:  "1min",
		types.Interval5m:  "5min",
		types.Interval15m: "15min",
		types.Interval30m: "30min",
		types.Interval1h:  "1h",
		types.Interval4h:  "4h",
		types.Interval6h:  "6Hutc",
		types.Interval12h: "12Hutc",
		types.Interval1d:  "1Dutc",
		types.Interval3d:  "3Dutc",
		types.Interval1w:  "1Wutc",
		types.Interval1mo: "1Mutc",
	}
)

func hasMaxDuration(interval types.Interval) (bool, time.Duration) {
	switch interval {
	case types.Interval1m, types.Interval5m:
		return true, 30 * 24 * time.Hour
	case types.Interval15m:
		return true, 52 * 24 * time.Hour
	case types.Interval30m:
		return true, 62 * 24 * time.Hour
	case types.Interval1h:
		return true, 83 * 24 * time.Hour
	case types.Interval4h:
		return true, 240 * 24 * time.Hour
	case types.Interval6h:
		return true, 360 * 24 * time.Hour
	default:
		return false, 0 * time.Duration(0)
	}
}

type KLine struct {
	StartTime    types.MillisecondTimestamp
	OpenPrice    fixedpoint.Value
	HighestPrice fixedpoint.Value
	LowestPrice  fixedpoint.Value
	ClosePrice   fixedpoint.Value
	Volume       fixedpoint.Value
	QuoteVolume  fixedpoint.Value
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
		QuoteVolume:              k.QuoteVolume,
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
		if len(raw) != 8 {
			return nil, fmt.Errorf("unexpected kline length: %d, data: %q", len(raw), raw)
		}
		// even though it supports 8 fields, we only parse the ones we need.
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
		if err = json.Unmarshal(raw[6], &kline.QuoteVolume); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into quote volume: %q", raw[6])
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

type Balance struct {
	Coin      string           `json:"coin"`
	Available fixedpoint.Value `json:"available"`
	// Amount of frozen assets Usually frozen when the order is placed
	Frozen fixedpoint.Value `json:"frozen"`
	// Amount of locked assets Locked assests required to become a fiat merchants, etc.
	Locked fixedpoint.Value `json:"locked"`
	// Restricted availability For spot copy trading
	LimitAvailable fixedpoint.Value           `json:"limitAvailable"`
	UpdatedTime    types.MillisecondTimestamp `json:"uTime"`
}

type AccountEvent struct {
	Balances []Balance

	// internal use
	actionType ActionType
	instId     string
}

type Trade struct {
	// Latest filled price
	FillPrice fixedpoint.Value `json:"fillPrice"`
	TradeId   strint.Int64     `json:"tradeId"`
	// Number of latest filled orders
	BaseVolume fixedpoint.Value           `json:"baseVolume"`
	FillTime   types.MillisecondTimestamp `json:"fillTime"`
	// Transaction fee of the latest transaction, negative value
	FillFee fixedpoint.Value `json:"fillFee"`
	// Currency of transaction fee of the latest transaction
	FillFeeCoin string `json:"fillFeeCoin"`
	// Direction of liquidity of the latest transaction
	TradeScope string `json:"tradeScope"`
}

type Order struct {
	Trade

	InstId string `json:"instId"`
	// OrderId are always numeric. It's confirmed with official customer service. https://t.me/bitgetOpenapi/24172
	OrderId       strint.Int64 `json:"orderId"`
	ClientOrderId string       `json:"clientOid"`
	// NewSize represents the order quantity, following the specified rules:
	// when orderType=limit, newSize represents the quantity of base coin,
	// when orderType=marketandside=buy, newSize represents the quantity of quote coin,
	// when orderType=marketandside=sell, newSize represents the quantity of base coin.
	NewSize fixedpoint.Value `json:"newSize"`
	// Buy amount, returned when buying at market price
	Notional      fixedpoint.Value `json:"notional"`
	OrderType     v2.OrderType     `json:"orderType"`
	Force         v2.OrderForce    `json:"force"`
	Side          v2.SideType      `json:"side"`
	AccBaseVolume fixedpoint.Value `json:"accBaseVolume"`
	PriceAvg      fixedpoint.Value `json:"priceAvg"`
	// The Price field is only applicable to limit orders.
	Price       fixedpoint.Value           `json:"price"`
	Status      v2.OrderStatus             `json:"status"`
	CreatedTime types.MillisecondTimestamp `json:"cTime"`
	UpdatedTime types.MillisecondTimestamp `json:"uTime"`
	FeeDetail   []struct {
		FeeCoin string `json:"feeCoin"`
		Fee     string `json:"fee"`
	} `json:"feeDetail"`
	EnterPointSource string `json:"enterPointSource"`
}

func (o *Order) isMaker() (bool, error) {
	switch o.TradeScope {
	case "T":
		return false, nil
	case "M":
		return true, nil
	default:
		return false, fmt.Errorf("unexpected trade scope: %s", o.TradeScope)
	}
}

type OrderTradeEvent struct {
	Orders []Order

	// internal use
	actionType ActionType
	instId     string
}
