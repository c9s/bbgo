package ftx

import "encoding/json"

type operation string

const subscribe operation = "subscribe"
const unsubscribe operation = "unsubscribe"

type channel string

const orderbook channel = "orderbook"
const trades channel = "trades"
const ticker channel = "ticker"

// {'op': 'subscribe', 'channel': 'trades', 'market': 'BTC-PERP'}
type SubscribeRequest struct {
	Operation operation `json:"op"`
	Channel   channel   `json:"channel"`
	Market    string    `json:"market"`
}

type respType string

const errRespType respType = "error"
const subscribedRespType respType = "subscribed"
const unsubscribedRespType respType = "unsubscribed"
const infoRespType respType = "info"
const partialRespType respType = "partial"
const updateRespType respType = "update"

type mandatoryFields struct {
	Type respType `json:"type"`

	// Channel is mandatory
	Channel channel `json:"channel"`

	// Market is mandatory
	Market string `json:"market"`
}

// doc: https://docs.ftx.com/#response-format
type rawResponse struct {
	mandatoryFields

	// The following fields are optional.
	// Example 1: {"type": "error", "code": 404, "msg": "No such market: BTCUSDT"}
	Code    int64                      `json:"code"`
	Message string                     `json:"msg"`
	Data    map[string]json.RawMessage `json:"data"`
}

func (r rawResponse) toSubscribedResp() subscribedResponse {
	return subscribedResponse{
		mandatoryFields: r.mandatoryFields,
	}
}

// {"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}
type subscribedResponse struct {
	mandatoryFields
}
