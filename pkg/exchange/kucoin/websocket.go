package kucoin

import (
	"encoding/json"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type WebSocketMessageType string

const (
	WebSocketMessageTypePing        WebSocketMessageType = "ping"
	WebSocketMessageTypeSubscribe   WebSocketMessageType = "subscribe"
	WebSocketMessageTypeUnsubscribe WebSocketMessageType = "unsubscribe"
	WebSocketMessageTypeAck         WebSocketMessageType = "ack"
	WebSocketMessageTypeError       WebSocketMessageType = "error"
	WebSocketMessageTypePong        WebSocketMessageType = "pong"
	WebSocketMessageTypeWelcome     WebSocketMessageType = "welcome"
	WebSocketMessageTypeMessage     WebSocketMessageType = "message"
)

type WebSocketSubject string

const (
	WebSocketSubjectTradeTicker        WebSocketSubject = "trade.ticker"
	WebSocketSubjectTradeSnapshot      WebSocketSubject = "trade.snapshot" // ticker snapshot
	WebSocketSubjectTradeL2Update      WebSocketSubject = "trade.l2update" // order book L2
	WebSocketSubjectLevel2             WebSocketSubject = "level2"         // level2
	WebSocketSubjectTradeCandlesUpdate WebSocketSubject = "trade.candles.update"
	WebSocketSubjectTradeCandlesAdd    WebSocketSubject = "trade.candles.add"

	// private subjects
	WebSocketSubjectOrderChange    WebSocketSubject = "orderChange"
	WebSocketSubjectAccountBalance WebSocketSubject = "account.balance"
	WebSocketSubjectStopOrder      WebSocketSubject = "stopOrder"
)

type WebSocketCommand struct {
	Id             int64                `json:"id"`
	Type           WebSocketMessageType `json:"type"`
	Topic          string               `json:"topic"`
	PrivateChannel bool                 `json:"privateChannel"`
	Response       bool                 `json:"response"`
}

func (c *WebSocketCommand) JSON() ([]byte, error) {
	type tt WebSocketCommand
	var a = (*tt)(c)
	return json.Marshal(a)
}

type WebSocketEvent struct {
	Type    WebSocketMessageType `json:"type"`
	Topic   string               `json:"topic"`
	Subject WebSocketSubject     `json:"subject"`
	Data    json.RawMessage      `json:"data"`
	Code    int                  `json:"code"` // used in type error

	// Object is used for storing the parsed Data
	Object interface{} `json:"-"`
}

type WebSocketTickerEvent struct {
	Sequence    string           `json:"sequence"`
	Price       fixedpoint.Value `json:"price"`
	Size        fixedpoint.Value `json:"size"`
	BestAsk     fixedpoint.Value `json:"bestAsk"`
	BestAskSize fixedpoint.Value `json:"bestAskSize"`
	BestBid     fixedpoint.Value `json:"bestBid"`
	BestBidSize fixedpoint.Value `json:"bestBidSize"`
}

type WebSocketOrderBookL2Event struct {
	SequenceStart int64  `json:"sequenceStart"`
	SequenceEnd   int64  `json:"sequenceEnd"`
	Symbol        string `json:"symbol"`
	Changes       struct {
		Asks types.PriceVolumeSlice `json:"asks"`
		Bids types.PriceVolumeSlice `json:"bids"`
	} `json:"changes"`
	Time types.MillisecondTimestamp `json:"time"`
}

type WebSocketCandleEvent struct {
	Symbol  string                     `json:"symbol"`
	Candles []string                   `json:"candles"`
	Time    types.MillisecondTimestamp `json:"time"`

	// Interval is an injected field (not from the payload)
	Interval types.Interval

	// Is a new candle or not
	Add bool
}

func (e *WebSocketCandleEvent) KLine() types.KLine {
	startTime := types.MustParseUnixTimestamp(e.Candles[0])
	openPrice := fixedpoint.MustNewFromString(e.Candles[1])
	closePrice := fixedpoint.MustNewFromString(e.Candles[2])
	highPrice := fixedpoint.MustNewFromString(e.Candles[3])
	lowPrice := fixedpoint.MustNewFromString(e.Candles[4])
	volume := fixedpoint.MustNewFromString(e.Candles[5])
	quoteVolume := fixedpoint.MustNewFromString(e.Candles[6])
	kline := types.KLine{
		Exchange:    types.ExchangeKucoin,
		Symbol:      toGlobalSymbol(e.Symbol),
		StartTime:   types.Time(startTime),
		EndTime:     types.Time(startTime.Add(e.Interval.Duration() - time.Millisecond)),
		Interval:    e.Interval,
		Open:        openPrice,
		Close:       closePrice,
		High:        highPrice,
		Low:         lowPrice,
		Volume:      volume,
		QuoteVolume: quoteVolume,
		Closed:      false,
	}
	return kline
}

type WebSocketPrivateOrderEvent struct {
	OrderId    string                    `json:"orderId"`
	TradeId    string                    `json:"tradeId"`
	Symbol     string                    `json:"symbol"`
	OrderType  string                    `json:"orderType"`
	Side       string                    `json:"side"`
	Type       string                    `json:"type"`
	OrderTime  types.NanosecondTimestamp `json:"orderTime"`
	Price      fixedpoint.Value          `json:"price"`
	Size       fixedpoint.Value          `json:"size"`
	FilledSize fixedpoint.Value          `json:"filledSize"`
	RemainSize fixedpoint.Value          `json:"remainSize"`

	Liquidity  string                     `json:"liquidity"`
	MatchPrice fixedpoint.Value           `json:"matchPrice"`
	MatchSize  fixedpoint.Value           `json:"matchSize"`
	ClientOid  string                     `json:"clientOid"`
	Status     string                     `json:"status"`
	Ts         types.MillisecondTimestamp `json:"ts"`
}

type WebSocketAccountBalanceEvent struct {
	Total           fixedpoint.Value `json:"total"`
	Available       fixedpoint.Value `json:"available"`
	AvailableChange fixedpoint.Value `json:"availableChange"`
	Currency        string           `json:"currency"`
	Hold            fixedpoint.Value `json:"hold"`
	HoldChange      fixedpoint.Value `json:"holdChange"`
	RelationEvent   string           `json:"relationEvent"`
	RelationEventId string           `json:"relationEventId"`
	RelationContext struct {
		Symbol  string `json:"symbol"`
		TradeId string `json:"tradeId"`
		OrderId string `json:"orderId"`
	} `json:"relationContext"`
	Time string `json:"time"`
}
