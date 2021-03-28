package ftx

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type operation string

const login operation = "login"
const subscribe operation = "subscribe"
const unsubscribe operation = "unsubscribe"

type channel string

const orderBookChannel channel = "orderbook"
const privateOrdersChannel channel = "orders"
const privateTradesChannel channel = "fills"

var errUnsupportedConversion = fmt.Errorf("unsupported conversion")

/*
Private:
	order update: `{'op': 'subscribe', 'channel': 'orders'}`
	login: `{"args": { "key": "<api_key>", "sign": "<signature>", "time": <ts> }, "op": "login" }`

*/
type websocketRequest struct {
	Operation operation `json:"op"`

	// {'op': 'subscribe', 'channel': 'trades', 'market': 'BTC-PERP'}
	Channel channel `json:"channel,omitempty"`
	Market  string  `json:"market,omitempty"`

	Login loginArgs `json:"args,omitempty"`
}

/*
{
  "args": {
    "key": "<api_key>",
    "sign": "<signature>",
    "time": <ts>
  },
  "op": "login"
}
*/
type loginArgs struct {
	Key       string `json:"key"`
	Signature string `json:"sign"`
	Time      int64  `json:"time"`
}

func newLoginRequest(key, secret string, t time.Time) websocketRequest {
	millis := t.UnixNano() / int64(time.Millisecond)
	return websocketRequest{
		Operation: login,
		Login: loginArgs{
			Key:       key,
			Signature: sign(secret, loginBody(millis)),
			Time:      millis,
		},
	}
}

func loginBody(millis int64) string {
	return fmt.Sprintf("%dwebsocket_login", millis)
}

type respType string

const errRespType respType = "error"
const subscribedRespType respType = "subscribed"
const unsubscribedRespType respType = "unsubscribed"
const infoRespType respType = "info"
const partialRespType respType = "partial"
const updateRespType respType = "update"

type websocketResponse struct {
	mandatoryFields

	optionalFields
}

type mandatoryFields struct {
	Channel channel `json:"channel"`

	Type respType `json:"type"`
}

type optionalFields struct {
	Market string `json:"market"`

	// Example: {"type": "error", "code": 404, "msg": "No such market: BTCUSDT"}
	Code int64 `json:"code"`

	Message string `json:"msg"`

	Data json.RawMessage `json:"data"`
}

type orderUpdateResponse struct {
	mandatoryFields

	Data order `json:"data"`
}

func (r websocketResponse) toOrderUpdateResponse() (orderUpdateResponse, error) {
	if r.Channel != privateOrdersChannel {
		return orderUpdateResponse{}, fmt.Errorf("type %s, channel %s: %w", r.Type, r.Channel, errUnsupportedConversion)
	}
	var o orderUpdateResponse
	if err := json.Unmarshal(r.Data, &o.Data); err != nil {
		return orderUpdateResponse{}, err
	}
	o.mandatoryFields = r.mandatoryFields
	return o, nil
}

type tradeUpdateResponse struct {
	mandatoryFields

	Data fill `json:"data"`
}

func (r websocketResponse) toTradeUpdateResponse() (tradeUpdateResponse, error) {
	if r.Channel != privateTradesChannel {
		return tradeUpdateResponse{}, fmt.Errorf("type %s, channel %s: %w", r.Type, r.Channel, errUnsupportedConversion)
	}
	var t tradeUpdateResponse
	if err := json.Unmarshal(r.Data, &t.Data); err != nil {
		return tradeUpdateResponse{}, err
	}
	t.mandatoryFields = r.mandatoryFields
	return t, nil
}

/*
Private:
	order: {"type": "subscribed", "channel": "orders"}

Public
	orderbook: {"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}

*/
type subscribedResponse struct {
	mandatoryFields

	Market string `json:"market"`
}

func (s subscribedResponse) String() string {
	return fmt.Sprintf("`%s` channel is subsribed", strings.TrimSpace(fmt.Sprintf("%s %s", s.Market, s.Channel)))
}

// {"type": "subscribed", "channel": "orderbook", "market": "BTC/USDT"}
func (r websocketResponse) toSubscribedResponse() (subscribedResponse, error) {
	if r.Type != subscribedRespType {
		return subscribedResponse{}, fmt.Errorf("type %s, channel %s: %w", r.Type, r.Channel, errUnsupportedConversion)
	}

	return subscribedResponse{
		mandatoryFields: r.mandatoryFields,
		Market:          r.Market,
	}, nil
}

// {"type": "error", "code": 400, "msg": "Already logged in"}
type errResponse struct {
	Code    int64  `json:"code"`
	Message string `json:"msg"`
}

func (e errResponse) String() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

func (r websocketResponse) toErrResponse() errResponse {
	return errResponse{
		Code:    r.Code,
		Message: r.Message,
	}
}

func (r websocketResponse) toPublicOrderBookResponse() (orderBookResponse, error) {
	if r.Channel != orderBookChannel {
		return orderBookResponse{}, fmt.Errorf("type %s, channel %s: %w", r.Type, r.Channel, errUnsupportedConversion)
	}

	var o orderBookResponse
	if err := json.Unmarshal(r.Data, &o); err != nil {
		return orderBookResponse{}, err
	}

	o.mandatoryFields = r.mandatoryFields
	o.Market = r.Market
	o.Timestamp = nanoToTime(o.Time)

	return o, nil
}

func nanoToTime(input float64) time.Time {
	sec, dec := math.Modf(input)
	return time.Unix(int64(sec), int64(dec*1e9))
}

type orderBookResponse struct {
	mandatoryFields

	Market string `json:"market"`

	Action string `json:"action"`

	Time float64 `json:"time"`

	Timestamp time.Time

	Checksum uint32 `json:"checksum"`

	// best 100 orders. Ex. {[100,1], [50, 2]}
	Bids [][]json.Number `json:"bids"`

	// best 100 orders. Ex. {[51, 1], [102, 3]}
	Asks [][]json.Number `json:"asks"`
}

// only 100 orders so we use linear search here
func (r *orderBookResponse) update(orderUpdates orderBookResponse) {
	r.Checksum = orderUpdates.Checksum
	r.updateBids(orderUpdates.Bids)
	r.updateAsks(orderUpdates.Asks)
}

func (r *orderBookResponse) updateAsks(asks [][]json.Number) {
	higherPrice := func(dst, src float64) bool {
		return dst < src
	}
	for _, o := range asks {
		if remove := o[1] == "0"; remove {
			r.Asks = removePrice(r.Asks, o[0])
		} else {
			r.Asks = upsertPriceVolume(r.Asks, o, higherPrice)
		}
	}
}

func (r *orderBookResponse) updateBids(bids [][]json.Number) {
	lessPrice := func(dst, src float64) bool {
		return dst > src
	}
	for _, o := range bids {
		if remove := o[1] == "0"; remove {
			r.Bids = removePrice(r.Bids, o[0])
		} else {
			r.Bids = upsertPriceVolume(r.Bids, o, lessPrice)
		}
	}
}

func upsertPriceVolume(dst [][]json.Number, src []json.Number, priceComparator func(dst float64, src float64) bool) [][]json.Number {
	for i, pv := range dst {
		dstPrice := pv[0]
		srcPrice := src[0]

		// update volume
		if dstPrice == srcPrice {
			pv[1] = src[1]
			return dst
		}

		// The value must be a number which is verified by json.Unmarshal, so the err
		// should never happen.
		dstPriceNum, err := strconv.ParseFloat(string(dstPrice), 64)
		if err != nil {
			logger.WithError(err).Errorf("unexpected price %s", dstPrice)
			continue
		}
		srcPriceNum, err := strconv.ParseFloat(string(srcPrice), 64)
		if err != nil {
			logger.WithError(err).Errorf("unexpected price updates %s", srcPrice)
			continue
		}

		if !priceComparator(dstPriceNum, srcPriceNum) {
			return insertAt(dst, i, src)
		}
	}

	return append(dst, src)
}

func insertAt(dst [][]json.Number, id int, pv []json.Number) (result [][]json.Number) {
	result = append(result, dst[:id]...)
	result = append(result, pv)
	result = append(result, dst[id:]...)
	return
}

func removePrice(dst [][]json.Number, price json.Number) [][]json.Number {
	for i, pv := range dst {
		if pv[0] == price {
			return append(dst[:i], dst[i+1:]...)
		}
	}

	return dst
}

func (r orderBookResponse) verifyChecksum() error {
	if crc32Val := crc32.ChecksumIEEE([]byte(checksumString(r.Bids, r.Asks))); crc32Val != r.Checksum {
		return fmt.Errorf("expected checksum %d, actual checksum %d: %w", r.Checksum, crc32Val, errUnmatchedChecksum)
	}
	return nil
}

// <best_bid_price>:<best_bid_size>:<best_ask_price>:<best_ask_size>...
func checksumString(bids, asks [][]json.Number) string {
	sb := strings.Builder{}
	appendNumber := func(pv []json.Number) {
		if sb.Len() != 0 {
			sb.WriteString(":")
		}
		sb.WriteString(string(pv[0]))
		sb.WriteString(":")
		sb.WriteString(string(pv[1]))
	}

	bidsLen := len(bids)
	asksLen := len(asks)
	for i := 0; i < bidsLen || i < asksLen; i++ {
		if i < bidsLen {
			appendNumber(bids[i])
		}
		if i < asksLen {
			appendNumber(asks[i])
		}
	}
	return sb.String()
}

var errUnmatchedChecksum = fmt.Errorf("unmatched checksum")

func toGlobalOrderBook(r orderBookResponse) (types.OrderBook, error) {
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
