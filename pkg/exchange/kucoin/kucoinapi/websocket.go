package kucoinapi

import (
	"encoding/json"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type WebSocketMessageType string

const (
	WebSocketMessageTypePing        WebSocketMessageType = "ping"
	WebSocketMessageTypeSubscribe   WebSocketMessageType = "subscribe"
	WebSocketMessageTypeUnsubscribe WebSocketMessageType = "unsubscribe"
	WebSocketMessageTypeAck         WebSocketMessageType = "ack"
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

	// private subjects
	WebSocketSubjectOrderChange    WebSocketSubject = "orderChange"
	WebSocketSubjectAccountBalance WebSocketSubject = "account.balance"
	WebSocketSubjectStopOrder      WebSocketSubject = "stopOrder"
)

type WebSocketCommand struct {
	Id             int64  `json:"id"`
	Type           WebSocketMessageType `json:"type"`
	Topic          string `json:"topic"`
	PrivateChannel bool   `json:"privateChannel"`
	Response       bool   `json:"response"`
}

func (c *WebSocketCommand) JSON() ([]byte, error) {
	type tt WebSocketCommand
	var a = (*tt)(c)
	return json.Marshal(a)
}

type WebSocketResponse struct {
	Type    WebSocketMessageType `json:"type"`
	Topic   string               `json:"topic"`
	Subject WebSocketSubject     `json:"subject"`
	Data    json.RawMessage      `json:"data"`

	// Object is used for storing the parsed Data
	Object interface{} `json:"-"`
}

type WebSocketTicker struct {
	Sequence    string           `json:"sequence"`
	Price       fixedpoint.Value `json:"price"`
	Size        fixedpoint.Value `json:"size"`
	BestAsk     fixedpoint.Value `json:"bestAsk"`
	BestAskSize fixedpoint.Value `json:"bestAskSize"`
	BestBid     fixedpoint.Value `json:"bestBid"`
	BestBidSize fixedpoint.Value `json:"bestBidSize"`
}

type WebSocketOrderBookL2 struct {
	SequenceStart int64  `json:"sequenceStart"`
	SequenceEnd   int64  `json:"sequenceEnd"`
	Symbol        string `json:"symbol"`
	Changes       struct {
		Asks [][]string `json:"asks"`
		Bids [][]string `json:"bids"`
	} `json:"changes"`
}

type WebSocketKLine struct {
	Symbol  string   `json:"symbol"`
	Candles []string `json:"candles"`
	Time    int64    `json:"time"`
}

type WebSocketPrivateOrder struct {
	Symbol     string                     `json:"symbol"`
	OrderType  string                     `json:"orderType"`
	Side       string                     `json:"side"`
	OrderId    string                     `json:"orderId"`
	Type       string                     `json:"type"`
	OrderTime  types.MillisecondTimestamp `json:"orderTime"`
	Price      fixedpoint.Value           `json:"price"`
	Size       fixedpoint.Value           `json:"size"`
	FilledSize fixedpoint.Value           `json:"filledSize"`
	RemainSize fixedpoint.Value           `json:"remainSize"`
	ClientOid  string                     `json:"clientOid"`
	Status     string                     `json:"status"`
	Ts         types.MillisecondTimestamp `json:"ts"`
}

type WebSocketAccountBalance struct {
	Total           fixedpoint.Value `json:"total"`
	Available       fixedpoint.Value `json:"available"`
	AvailableChange fixedpoint.Value `json:"availableChange"`
	Currency        string `json:"currency"`
	Hold            fixedpoint.Value `json:"hold"`
	HoldChange      fixedpoint.Value `json:"holdChange"`
	RelationEvent   string `json:"relationEvent"`
	RelationEventId string `json:"relationEventId"`
	RelationContext struct {
		Symbol  string `json:"symbol"`
		TradeId string `json:"tradeId"`
		OrderId string `json:"orderId"`
	} `json:"relationContext"`
	Time string `json:"time"`
}
