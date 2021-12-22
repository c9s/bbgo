package kucoinapi

import "encoding/json"

type WebSocketCommand struct {
    Id             int64  `json:"id"`
    Type           string `json:"type"`
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
    Type    string `json:"type"`
    Topic   string `json:"topic"`
    Subject string `json:"subject"`
}

type WebSocketTicker struct {
    Sequence    string `json:"sequence"`
    Price       string `json:"price"`
    Size        string `json:"size"`
    BestAsk     string `json:"bestAsk"`
    BestAskSize string `json:"bestAskSize"`
    BestBid     string `json:"bestBid"`
    BestBidSize string `json:"bestBidSize"`
}

type WebSocketOrderBook struct {
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
    Symbol     string `json:"symbol"`
    OrderType  string `json:"orderType"`
    Side       string `json:"side"`
    OrderId    string `json:"orderId"`
    Type       string `json:"type"`
    OrderTime  int64  `json:"orderTime"`
    Size       string `json:"size"`
    FilledSize string `json:"filledSize"`
    Price      string `json:"price"`
    ClientOid  string `json:"clientOid"`
    RemainSize string `json:"remainSize"`
    Status     string `json:"status"`
    Ts         int64  `json:"ts"`
}

type WebSocketAccountBalance struct {
    Total           string `json:"total"`
    Available       string `json:"available"`
    AvailableChange string `json:"availableChange"`
    Currency        string `json:"currency"`
    Hold            string `json:"hold"`
    HoldChange      string `json:"holdChange"`
    RelationEvent   string `json:"relationEvent"`
    RelationEventId string `json:"relationEventId"`
    RelationContext struct {
        Symbol  string `json:"symbol"`
        TradeId string `json:"tradeId"`
        OrderId string `json:"orderId"`
    } `json:"relationContext"`
    Time string `json:"time"`
}