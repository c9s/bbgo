package backpackapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// WebsocketMethod is the method of a websocket request.
type WebsocketMethod string

const (
	WebsocketMethodSubscribe   WebsocketMethod = "SUBSCRIBE"
	WebsocketMethodUnsubscribe WebsocketMethod = "UNSUBSCRIBE"
)

// InstructionSubscribe is the instruction signed when subscribing to a private stream.
const InstructionSubscribe Instruction = "subscribe"

// WebsocketRequest is a subscribe or unsubscribe request.
//
// The signature field is only present when subscribing to a private stream. It carries the
// verifying key, the signature, the timestamp and the window, in that order.
type WebsocketRequest struct {
	Method WebsocketMethod `json:"method"`
	Params []string        `json:"params"`

	Signature []string `json:"signature,omitempty"`
}

// NewWebsocketSubscribeRequest builds a signed subscribe request for the given private streams.
//
// The signed payload has no parameters of its own, only the instruction, the timestamp and the
// window:
//
//	instruction=subscribe&timestamp=<ms>&window=<ms>
func (c *RestClient) NewWebsocketSubscribeRequest(streams []string) (WebsocketRequest, error) {
	if len(c.key) == 0 {
		return WebsocketRequest{}, errNoApiKey
	}

	if c.privateKey == nil {
		return WebsocketRequest{}, errNoApiSecret
	}

	timestamp := time.Now().UnixMilli()
	window := c.getWindow()

	signature := c.sign(buildSigningString(InstructionSubscribe, nil, timestamp, window))

	return WebsocketRequest{
		Method: WebsocketMethodSubscribe,
		Params: streams,
		Signature: []string{
			c.key,
			signature,
			strconv.FormatInt(timestamp, 10),
			strconv.FormatUint(window, 10),
		},
	}, nil
}

// NewWebsocketUnsubscribeRequest builds an unsubscribe request. Unsubscribing is never signed.
func NewWebsocketUnsubscribeRequest(streams []string) WebsocketRequest {
	return WebsocketRequest{
		Method: WebsocketMethodUnsubscribe,
		Params: streams,
	}
}

// Stream name prefixes. A stream is named "<type>.<symbol>", except the kline stream which is
// "kline.<interval>.<symbol>" and the account streams which have no symbol.
const (
	StreamBookTicker     = "bookTicker"
	StreamDepth          = "depth"
	StreamTrade          = "trade"
	StreamTicker         = "ticker"
	StreamKLine          = "kline"
	StreamMarkPrice      = "markPrice"
	StreamOrderUpdate    = "account.orderUpdate"
	StreamBalanceUpdate  = "account.balanceUpdate"
	StreamPositionUpdate = "account.positionUpdate"
)

// WebsocketEventType is the value of the "e" field of a stream payload.
type WebsocketEventType string

const (
	EventTypeBookTicker    WebsocketEventType = "bookTicker"
	EventTypeDepth         WebsocketEventType = "depth"
	EventTypeTrade         WebsocketEventType = "trade"
	EventTypeTicker        WebsocketEventType = "ticker"
	EventTypeKLine         WebsocketEventType = "kline"
	EventTypeMarkPrice     WebsocketEventType = "markPrice"
	EventTypeBalanceUpdate WebsocketEventType = "balanceUpdate"

	// the order update stream reports the life cycle stage in the event type
	EventTypeOrderAccepted  WebsocketEventType = "orderAccepted"
	EventTypeOrderCancelled WebsocketEventType = "orderCancelled"
	EventTypeOrderExpired   WebsocketEventType = "orderExpired"
	EventTypeOrderFill      WebsocketEventType = "orderFill"
	EventTypeOrderModified  WebsocketEventType = "orderModified"
	EventTypeTriggerPlaced  WebsocketEventType = "triggerPlaced"
	EventTypeTriggerFailed  WebsocketEventType = "triggerFailed"
)

// IsOrderUpdate reports whether the event belongs to the account.orderUpdate stream.
func (e WebsocketEventType) IsOrderUpdate() bool {
	switch e {
	case EventTypeOrderAccepted, EventTypeOrderCancelled, EventTypeOrderExpired,
		EventTypeOrderFill, EventTypeOrderModified, EventTypeTriggerPlaced, EventTypeTriggerFailed:
		return true
	}

	return false
}

// WebsocketError is the error frame the server sends for a rejected request, e.g.
// {"id":null,"error":{"code":4003,"message":"Invalid signature"}}
type WebsocketError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *WebsocketError) Error() string {
	return fmt.Sprintf("backpack websocket error: code=%d, message=%s", e.Code, e.Message)
}

// websocketEnvelope is the outer frame: every stream payload arrives wrapped in it.
type websocketEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
	Error  *WebsocketError `json:"error"`
}

// EventBase carries the fields every stream payload has.
type EventBase struct {
	EventType WebsocketEventType   `json:"e"`
	EventTime MicrosecondTimestamp `json:"E"`
}

// BookTickerEvent is a bookTicker.<symbol> payload.
type BookTickerEvent struct {
	EventBase

	Symbol string `json:"s"`

	AskPrice    fixedpoint.Value `json:"a"`
	AskQuantity fixedpoint.Value `json:"A"`
	BidPrice    fixedpoint.Value `json:"b"`
	BidQuantity fixedpoint.Value `json:"B"`

	UpdateId   int64                `json:"u"`
	EngineTime MicrosecondTimestamp `json:"T"`
}

// DepthEvent is a depth.<symbol> payload.
//
// The updates are incremental and carry a Binance style update id range, so they have to be
// applied on top of a REST snapshot.
type DepthEvent struct {
	EventBase

	Symbol string `json:"s"`

	Asks []PriceLevel `json:"a"`
	Bids []PriceLevel `json:"b"`

	FirstUpdateId int64                `json:"U"`
	LastUpdateId  int64                `json:"u"`
	EngineTime    MicrosecondTimestamp `json:"T"`
}

// TradeEvent is a trade.<symbol> payload.
type TradeEvent struct {
	EventBase

	Symbol string `json:"s"`

	Price    fixedpoint.Value `json:"p"`
	Quantity fixedpoint.Value `json:"q"`

	TradeId int64 `json:"t"`

	// IsBuyerMaker reports whether the buyer was the maker of the trade.
	IsBuyerMaker bool `json:"m"`

	MakerOrderId string `json:"b"`
	TakerOrderId string `json:"a"`

	EngineTime MicrosecondTimestamp `json:"T"`
}

// TickerEvent is a ticker.<symbol> payload.
type TickerEvent struct {
	EventBase

	Symbol string `json:"s"`

	Open  fixedpoint.Value `json:"o"`
	Close fixedpoint.Value `json:"c"`
	High  fixedpoint.Value `json:"h"`
	Low   fixedpoint.Value `json:"l"`

	Volume      fixedpoint.Value `json:"v"`
	QuoteVolume fixedpoint.Value `json:"V"`

	Trades int64 `json:"n"`
}

// KLineEvent is a kline.<interval>.<symbol> payload.
//
// The payload carries no interval, so Interval is filled in by the parser from the stream name.
// There is no quote volume either.
type KLineEvent struct {
	EventBase

	Symbol string `json:"s"`

	// StartTime and EndTime are naive UTC datetimes, e.g. "2026-09-02T16:43:00".
	StartTime types.Time `json:"t"`
	EndTime   types.Time `json:"T"`

	// the prices are null for an interval without trades
	Open  fixedpoint.Value `json:"o"`
	High  fixedpoint.Value `json:"h"`
	Low   fixedpoint.Value `json:"l"`
	Close fixedpoint.Value `json:"c"`

	Volume fixedpoint.Value `json:"v"`
	Trades int64            `json:"n"`

	Closed bool `json:"X"`

	// Interval is not part of the payload, the parser fills it from the stream name.
	Interval string `json:"-"`
}

// MarkPriceEvent is a markPrice.<symbol> payload.
type MarkPriceEvent struct {
	EventBase

	Symbol string `json:"s"`

	MarkPrice   fixedpoint.Value `json:"p"`
	IndexPrice  fixedpoint.Value `json:"i"`
	FundingRate fixedpoint.Value `json:"f"`

	// NextFundingTimestamp is in milliseconds, unlike the other timestamps.
	NextFundingTimestamp types.MillisecondTimestamp `json:"n"`

	EngineTime MicrosecondTimestamp `json:"T"`
}

// BalanceUpdateEvent is an account.balanceUpdate payload.
type BalanceUpdateEvent struct {
	EventBase

	Asset string `json:"a"`

	Available fixedpoint.Value `json:"A"`
	Locked    fixedpoint.Value `json:"L"`
	Staked    fixedpoint.Value `json:"S"`

	EngineTime MicrosecondTimestamp `json:"T"`
}

// OrderUpdateEvent is an account.orderUpdate payload.
//
// Note that the order type arrives upper cased here ("LIMIT"), unlike the REST API which spells
// it "Limit"; use WebsocketOrderType.OrderType to normalize it.
type OrderUpdateEvent struct {
	EventBase

	Symbol  string `json:"s"`
	OrderId string `json:"i"`

	Side      Side               `json:"S"`
	OrderType WebsocketOrderType `json:"o"`
	Status    OrderStatus        `json:"X"`

	Price    fixedpoint.Value `json:"p"`
	Quantity fixedpoint.Value `json:"q"`

	// QuoteQuantity is the notional of the order.
	QuoteQuantity fixedpoint.Value `json:"Q"`

	ExecutedQuantity      fixedpoint.Value `json:"z"`
	ExecutedQuoteQuantity fixedpoint.Value `json:"Z"`

	// the fill fields are only set on an orderFill event
	FillPrice    fixedpoint.Value `json:"L"`
	FillQuantity fixedpoint.Value `json:"l"`
	Fee          fixedpoint.Value `json:"n"`
	FeeSymbol    string           `json:"N"`
	IsMaker      bool             `json:"m"`

	// TradeId is null unless the event is a fill.
	TradeId *int64 `json:"t"`

	TimeInForce         TimeInForce         `json:"f"`
	SelfTradePrevention SelfTradePrevention `json:"V"`

	ReduceOnly bool `json:"r"`
	PostOnly   bool `json:"y"`

	ClientOrderId uint32 `json:"c"`

	// Origin is USER for a client order, or one of the liquidation/ADL values.
	Origin string `json:"O"`

	ExpiryReason string `json:"er"`

	EngineTime MicrosecondTimestamp `json:"T"`
}

// WebsocketOrderType is the order type as spelled by the websocket API.
//
// The REST API returns "Limit" and "Market" while the websocket returns "LIMIT" and "MARKET",
// so the value is normalized rather than compared directly.
type WebsocketOrderType string

// OrderType converts the websocket spelling into the REST OrderType.
func (t WebsocketOrderType) OrderType() OrderType {
	switch strings.ToUpper(string(t)) {
	case "LIMIT":
		return OrderTypeLimit
	case "MARKET":
		return OrderTypeMarket
	}

	return OrderType(t)
}

// ParseWebsocketMessage decodes a raw websocket frame into one of the event types above.
//
// It returns a *WebsocketError for an error frame, and (nil, nil) for a frame that carries no
// recognized payload, which the caller is expected to ignore.
func ParseWebsocketMessage(message []byte) (interface{}, error) {
	var envelope websocketEnvelope
	if err := json.Unmarshal(message, &envelope); err != nil {
		return nil, err
	}

	if envelope.Error != nil {
		return envelope.Error, nil
	}

	if len(envelope.Data) == 0 {
		return nil, nil
	}

	var base EventBase
	if err := json.Unmarshal(envelope.Data, &base); err != nil {
		return nil, err
	}

	switch {
	case base.EventType == EventTypeBookTicker:
		return unmarshalEvent[BookTickerEvent](envelope.Data)

	case base.EventType == EventTypeDepth:
		return unmarshalEvent[DepthEvent](envelope.Data)

	case base.EventType == EventTypeTrade:
		return unmarshalEvent[TradeEvent](envelope.Data)

	case base.EventType == EventTypeTicker:
		return unmarshalEvent[TickerEvent](envelope.Data)

	case base.EventType == EventTypeMarkPrice:
		return unmarshalEvent[MarkPriceEvent](envelope.Data)

	case base.EventType == EventTypeBalanceUpdate:
		return unmarshalEvent[BalanceUpdateEvent](envelope.Data)

	case base.EventType == EventTypeKLine:
		event, err := unmarshalEvent[KLineEvent](envelope.Data)
		if err != nil {
			return nil, err
		}

		// the interval only exists in the stream name: "kline.<interval>.<symbol>"
		event.Interval = klineIntervalFromStream(envelope.Stream)
		return event, nil

	case base.EventType.IsOrderUpdate():
		return unmarshalEvent[OrderUpdateEvent](envelope.Data)
	}

	return nil, nil
}

func unmarshalEvent[T any](data []byte) (*T, error) {
	var event T
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("unable to unmarshal %T: %w, payload: %s", event, err, string(data))
	}

	return &event, nil
}

// klineIntervalFromStream extracts the interval from a "kline.<interval>.<symbol>" stream name.
func klineIntervalFromStream(stream string) string {
	parts := strings.SplitN(stream, ".", 3)
	if len(parts) < 3 {
		return ""
	}

	return parts[1]
}
