package ftx

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
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
	Code    int64           `json:"code"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
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

	if err := json.Unmarshal(r.Data, &o); err != nil {
		return snapshotResponse{}, err
	}

	sec, dec := math.Modf(o.Time)
	o.Timestamp = time.Unix(int64(sec), int64(dec*1e9))

	return o, nil
}

// {"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}
type subscribedResponse struct {
	mandatoryFields
}

type snapshotResponse struct {
	mandatoryFields

	Action string `json:"action"`

	Time float64 `json:"time"`

	Timestamp time.Time

	Checksum int64 `json:"checksum"`

	// Best 100 orders
	Bids [][]float64 `json:"bids"`

	// Best 100 orders
	Asks [][]float64 `json:"asks"`
}

func (r snapshotResponse) toGlobalOrderBook() types.OrderBook {
	return types.OrderBook{
		// ex. BTC/USDT
		Symbol: strings.ToUpper(r.Market),
		Bids:   toPriceVolumeSlice(r.Bids),
		Asks:   toPriceVolumeSlice(r.Asks),
	}
}

func toPriceVolumeSlice(orders [][]float64) types.PriceVolumeSlice {
	var pv types.PriceVolumeSlice
	for _, o := range orders {
		pv = append(pv, types.PriceVolume{
			Price:  fixedpoint.NewFromFloat(o[0]),
			Volume: fixedpoint.NewFromFloat(o[1]),
		})
	}
	return pv
}
