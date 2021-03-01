package ftx

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

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

func (r rawResponse) toSnapshotResp() (snapshotResponse, error) {
	o := snapshotResponse{
		mandatoryFields: r.mandatoryFields,
	}

	if err := json.Unmarshal(r.Data["action"], &o.Action); err != nil {
		return snapshotResponse{}, fmt.Errorf("failed to unmarshal data.action field: %w", err)
	}

	var t float64
	if err := json.Unmarshal(r.Data["time"], &t); err != nil {
		return snapshotResponse{}, fmt.Errorf("failed to unmarshal data.time field: %w", err)
	}
	sec, dec := math.Modf(t)
	o.Time = time.Unix(int64(sec), int64(dec*1e9))

	if err := json.Unmarshal(r.Data["checksum"], &o.Checksum); err != nil {
		return snapshotResponse{}, fmt.Errorf("failed to unmarshal data.checksum field: %w", err)
	}

	if err := json.Unmarshal(r.Data["bids"], &o.Bids); err != nil {
		return snapshotResponse{}, fmt.Errorf("failed to unmarshal data.bids field: %w", err)
	}

	if err := json.Unmarshal(r.Data["asks"], &o.Asks); err != nil {
		return snapshotResponse{}, fmt.Errorf("failed to unmarshal data.asks field: %w", err)
	}

	return o, nil
}

// {"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}
type subscribedResponse struct {
	mandatoryFields
}

type snapshotResponse struct {
	mandatoryFields

	Action string

	Time time.Time

	Checksum int64

	// Best 100 orders
	Bids [][]float64

	// Best 100 orders
	Asks [][]float64
}
