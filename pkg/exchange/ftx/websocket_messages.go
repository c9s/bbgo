package ftx

import (
	"encoding/json"
	"fmt"
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

func (r rawResponse) toDataResponse() (dataResponse, error) {
	o := dataResponse{
		mandatoryFields: r.mandatoryFields,
	}

	if err := json.Unmarshal(r.Data, &o); err != nil {
		return dataResponse{}, err
	}

	sec, dec := math.Modf(o.Time)
	o.Timestamp = time.Unix(int64(sec), int64(dec*1e9))

	return o, nil
}

// {"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}
type subscribedResponse struct {
	mandatoryFields
}

type dataResponse struct {
	mandatoryFields

	Action string `json:"action"`

	Time float64 `json:"time"`

	Timestamp time.Time

	Checksum int64 `json:"checksum"`

	Bids [][]json.Number `json:"bids"`

	Asks [][]json.Number `json:"asks"`
}

func toGlobalOrderBook(r dataResponse) (types.OrderBook, error) {
	bids, err := toPriceVolumeSlice(r.Bids)
	if err != nil {
		return types.OrderBook{}, fmt.Errorf("can't convert bids to priceVolumeSlice: %w", err)
	}
	asks, err := toPriceVolumeSlice(r.Asks)
	if err != nil {
		return types.OrderBook{}, fmt.Errorf("can't convert asks to priceVolumeSlice: %w", err)
	}
	return types.OrderBook{
		// ex. BTC/USDT
		Symbol: strings.ToUpper(r.Market),
		Bids:   bids,
		Asks:   asks,
	}, nil
}

func toPriceVolumeSlice(orders [][]json.Number) (types.PriceVolumeSlice, error) {
	var pv types.PriceVolumeSlice
	for _, o := range orders {
		p, err := fixedpoint.NewFromString(string(o[0]))
		if err != nil {
			return nil, fmt.Errorf("can't convert price %+v to fixedpoint: %w", o[0], err)
		}
		v, err := fixedpoint.NewFromString(string(o[1]))
		if err != nil {
			return nil, fmt.Errorf("can't convert volume %+v to fixedpoint: %w", o[0], err)
		}
		pv = append(pv, types.PriceVolume{Price: p, Volume: v})
	}
	return pv, nil
}
