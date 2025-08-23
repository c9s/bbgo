package bfxapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithFields(logrus.Fields{"exchange": "bitfinex", "module": "bfxapi"})

/*
Public channels should use the domain:
wss://api-pub.bitfinex.com/

The domain:
wss://api.bitfinex.com/
Should only be used for channels that require authentication.

The rate limit for the wss://api-pub.bitfinex.com/ domain
is set at 20 connections per minute.
*/
const PublicWebSocketURL = "wss://api-pub.bitfinex.com/ws/2"
const PrivateWebSocketURL = "wss://api.bitfinex.com/ws/2"

type Channel string

const (
	ChannelTicker  Channel = "ticker"
	ChannelTrades  Channel = "trades"
	ChannelBook    Channel = "book"
	ChannelCandles Channel = "candles"
	ChannelStatus  Channel = "status"
)

// Filter represents Bitfinex WebSocket auth filter type.
type Filter string

const (
	FilterTrading       Filter = "trading"
	FilterTradingBTCUSD Filter = "trading-tBTCUSD"
	FilterFunding       Filter = "funding"
	FilterFundingBTC    Filter = "funding-fBTC"
	FilterFundingUSD    Filter = "funding-fUSD"
	FilterFundingUST    Filter = "funding-fUST"

	FilterWallet            Filter = "wallet"
	FilterWalletExchangeBTC Filter = "wallet-exchange-BTC"

	FilterAlgo    Filter = "algo"
	FilterBalance Filter = "balance"
	FilterNotify  Filter = "notify"
)

type WebSocketRequest struct {
	Event   string  `json:"event"`
	Channel Channel `json:"channel"`
	Symbol  string  `json:"symbol"`

	Prec string `json:"prec,omitempty"`

	Key string `json:"key,omitempty"`
}

type WebSocketResponse struct {
	Event string `json:"event"`

	//
	Channel Channel `json:"channel"`

	// ChanId is the identification number assigned to the channel for the duration of this connection.
	ChanId any    `json:"chanId"`
	Symbol string `json:"symbol"`

	// Pair is the trading pair for ticker and trades channels, e.g., "tBTCUSD".
	Pair string `json:"pair"`

	// Currency is the funding currency for funding ticker and book channels, e.g., "USD".
	//
	// {"event":"subscribed","channel":"ticker","chanId":232591,"symbol":"fUSD","currency":"USD"}
	Currency string `json:"currency"`

	// Key
	//
	// Derivative pair status
	//
	//   {"event":"subscribed","channel":"status","chanId":335856,"key":"deriv:tBTCF0:USTF0"}
	//
	// Liquidation feed
	//
	//   {"event":"subscribed","channel":"status","chanId":91684,"key":"liq:global"}
	Key string `json:"key,omitempty"`

	Version  int    `json:"version,omitempty"`
	ServerID string `json:"serverId,omitempty"`

	// Status = "OK" or "FAIL"
	Status string `json:"status"`

	UserId int64 `json:"userId"`

	// AuthID is an UUID like 'a26236f1-ef44-4671-be32-197ce190348f'
	AuthID string `json:"auth_id,omitempty"`

	Caps *AuthCaps `json:"caps,omitempty"`

	Message *string `json:"msg,omitempty"`

	Code int64 `json:"code,omitempty"`
}

type WalletSnapshotEvent struct {
	Wallets []Wallet
}

// TickerEvent represents a ticker update or snapshot event.
type TickerEvent struct {
	ChannelID int64

	Bid     fixedpoint.Value
	BidSize fixedpoint.Value

	Ask     fixedpoint.Value
	AskSize fixedpoint.Value

	DailyChange         fixedpoint.Value
	DailyChangeRelative fixedpoint.Value

	LastPrice fixedpoint.Value
	Volume    fixedpoint.Value

	High fixedpoint.Value
	Low  fixedpoint.Value
}

// FundingTickerEvent represents a funding ticker update or snapshot event.
type FundingTickerEvent struct {
	ChannelID int64

	FRR       fixedpoint.Value
	Bid       fixedpoint.Value
	BidPeriod int64
	BidSize   fixedpoint.Value

	Ask       fixedpoint.Value
	AskPeriod int64
	AskSize   fixedpoint.Value

	DailyChange         fixedpoint.Value
	DailyChangeRelative fixedpoint.Value
	LastPrice           fixedpoint.Value
	Volume              fixedpoint.Value
	High                fixedpoint.Value
	Low                 fixedpoint.Value

	_ any // placeholder
	_ any // placeholder

	FRRAmountAvailable fixedpoint.Value
}

// BookUpdateEvent represents an order book update or snapshot event.
type BookUpdateEvent struct {
	ChannelID int64

	Entry BookEntry
}

type BookSnapshotEvent struct {
	ChannelID int64
	Entries   []BookEntry
}

// FundingBookUpdateEvent represents a funding book entry event.
type FundingBookUpdateEvent struct {
	ChannelID int64
	Entry     FundingBookEntry
}

type FundingBookSnapshotEvent struct {
	ChannelID int64
	Entries   []FundingBookEntry
}

// CandleEvent represents a kline/candle update or snapshot event.
type CandleEvent struct {
	ChannelID int64
	Symbol    string
	MTS       types.MillisecondTimestamp
	Open      fixedpoint.Value
	Close     fixedpoint.Value
	High      fixedpoint.Value
	Low       fixedpoint.Value
	Volume    fixedpoint.Value
	Closed    bool
}

// StatusEvent represents a status channel event.
type StatusEvent struct {
	ChannelID            int64
	Symbol               string
	TimeMs               int64
	_                    any // placeholder
	DerivPrice           fixedpoint.Value
	SpotPrice            fixedpoint.Value
	_                    any // placeholder
	InsuranceFundBalance fixedpoint.Value
	_                    any // placeholder
	NextFundingEventTsMs int64
	NextFundingAccrued   fixedpoint.Value
	NextFundingStep      int64
	_                    any // placeholder
	CurrentFunding       fixedpoint.Value
	_                    any // placeholder
	_                    any // placeholder
	MarkPrice            fixedpoint.Value
	_                    any // placeholder
	_                    any // placeholder
	OpenInterest         fixedpoint.Value
	_                    any // placeholder
	_                    any // placeholder
	_                    any // placeholder
	_                    any // placeholder
	ClampMin             fixedpoint.Value
	ClampMax             fixedpoint.Value
}

// MarketTradeEvent represents a trade update or snapshot event.
type MarketTradeEvent struct {
	ChannelID int64

	ID     int64
	Time   types.MillisecondTimestamp
	Amount fixedpoint.Value
	Price  fixedpoint.Value
}

// FundingMarketTradeEvent represents a funding trade execution event ("fte") in trades channel and funding trade snapshot.
type FundingMarketTradeEvent struct {
	ChannelID int64
	ID        int64
	Time      types.MillisecondTimestamp

	Amount fixedpoint.Value
	Rate   fixedpoint.Value
	Period int64
}

// HeartBeatEvent represents a heartbeat event from Bitfinex WebSocket.
type HeartBeatEvent struct {
	ChannelID int64
	Channel   Channel
}

type Permission struct {
	Read  int `json:"read,omitempty"`
	Write int `json:"write,omitempty"`
}

// AuthCaps represents the capabilities in the Bitfinex auth response.
type AuthCaps struct {
	BfxPay     *Permission `json:"bfxpay"`
	Orders     *Permission `json:"orders"`
	Account    *Permission `json:"account"`
	Funding    *Permission `json:"funding"`
	History    *Permission `json:"history"`
	Wallets    *Permission `json:"wallets"`
	Settings   *Permission `json:"settings"`
	Withdraw   *Permission `json:"withdraw"`
	Positions  *Permission `json:"positions"`
	UIWithdraw *Permission `json:"ui_withdraw"`
}

func (c *AuthCaps) UnmarshalJSON(data []byte) error {
	var s string
	if data[0] == '"' && data[len(data)-1] == '"' {
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("unable to unmarshal auth capabilities from string: %w", err)
		}

		data = []byte(s)
	}

	type T AuthCaps
	var caps T
	if err := json.Unmarshal(data, &caps); err != nil {
		return fmt.Errorf("unable to unmarshal auth capabilities: %w", err)
	}

	*c = AuthCaps(caps)
	return nil
}

// Parser maintains channelID mapping and parses Bitfinex messages.
type Parser struct {
	mu         sync.RWMutex
	channelMap map[int64]Channel // channelID -> channelType
}

// NewParser creates a new Bitfinex Parser.
func NewParser() *Parser {
	return &Parser{
		channelMap: make(map[int64]Channel),
	}
}

// registerChannel registers a channelID and its type.
func (p *Parser) registerChannel(channelID int64, channelType Channel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.channelMap[channelID] = channelType
}

// unregisterChannel removes a channelID from the mapping.
func (p *Parser) unregisterChannel(channelID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.channelMap, channelID)
}

// getChannelType returns the channel type for a channelID.
func (p *Parser) getChannelType(channelID int64) (Channel, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	typeStr, ok := p.channelMap[channelID]
	return typeStr, ok
}

// isJSONObject returns true if the message starts with '{'.
func isJSONObject(message []byte) bool {
	return len(message) > 0 && message[0] == '{'
}

// isJSONArray returns true if the message starts with '['.
func isJSONArray(message []byte) bool {
	return len(message) > 0 && message[0] == '['
}

// Parse parses a Bitfinex websocket message.
func (p *Parser) Parse(message []byte) (interface{}, error) {
	if len(message) == 0 {
		return nil, nil
	}

	if isJSONObject(message) {
		return p.parseObjectMessage(message)
	}

	if isJSONArray(message) {
		return p.parseArrayMessage(message)
	}

	log.Warnf("unknown websocket message format: %s", string(message))
	return nil, nil
}

// parseObjectMessage parses a JSON object message (WebSocketResponse).
func (p *Parser) parseObjectMessage(message []byte) (interface{}, error) {
	var resp WebSocketResponse
	if err := json.Unmarshal(message, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse websocket response: %w", err)
	}

	switch resp.Event {
	case "auth", "error", "info":
		return &resp, nil
	}

	if resp.ChanId != nil && resp.Channel != "" {
		switch v := resp.ChanId.(type) {
		case int64:
			p.registerChannel(v, resp.Channel)
		case float64:
			p.registerChannel(int64(v), resp.Channel)
		case string:
			channelID, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				p.registerChannel(channelID, resp.Channel)
			}
		default:
			return nil, fmt.Errorf("unknown channel ID type: %T", v)
		}
	}
	return &resp, nil
}

// parseArrayMessage parses a JSON array message (channel events, heartbeat, etc).
func (p *Parser) parseArrayMessage(message []byte) (interface{}, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(message, &arr); err != nil {
		return nil, fmt.Errorf("failed to parse websocket response: %w", err)
	}
	if len(arr) < 2 {
		return nil, nil
	}
	var channelID int64
	if err := json.Unmarshal(arr[0], &channelID); err != nil {
		return nil, err
	}

	// Check if the first element is chanId for private messages
	if channelID == 0 {
		if len(arr) < 2 {
			return nil, fmt.Errorf("invalid private message format, input: %s", message)
		}

		var streamName string
		if err := json.Unmarshal(arr[1], &streamName); err != nil {
			return nil, fmt.Errorf("failed to parse stream name: %w", err)
		}

		switch StreamName(streamName) {
		case StreamOrderSnapshot:
			return parseUserOrderSnapshot(arr[2])

		case StreamOrderUpdate, StreamOrderNew, StreamOrderCancel:
			return parseUserOrderUpdate(arr[2])

		case StreamPositionSnapshot:
			return parseUserPositionSnapshot(arr[2])

		case StreamPositionUpdate, StreamPositionClose:
			return parseUserPosition(arr[2])

		case StreamTradeExecuted, StreamTradeUpdate:
			return parseUserTrade(arr[2])

		case StreamFundingOfferSnapshot:
			return parseFundingOfferSnapshot(arr[2])

		case StreamFundingOfferNew, StreamFundingOfferUpdate, StreamFundingOfferCancel:
			return parseFundingOfferUpdate(arr[2])

		case StreamFundingCreditSnapshot, StreamFundingCreditNew, StreamFundingCreditUpdate, StreamFundingCreditClose:
			return nil, nil

		case StreamFundingLoanSnapshot, StreamFundingLoanNew, StreamFundingLoanUpdate, StreamFundingLoanClose:
			return nil, nil

		case StreamWalletSnapshot:
			return parseWalletSnapshot(arr[2])

		case StreamWalletUpdate:
			return parseWallet(arr[2])

		case StreamBalanceUpdate:
			return parseBalanceUpdateEvent(arr[2])

		case StreamMarginInfoUpdate:

		case StreamFundingInfoUpdate:
			return parseFundingInfoEvent(arr[2])
		case StreamFundingTradeExecuted, StreamFundingTradeUpdate:
			return nil, nil

		case StreamNotification:
			return nil, nil

		case StreamHeartBeat:
			return nil, nil

		default:
			return nil, fmt.Errorf("unknown private stream name: %s", streamName)
		}
	}

	channelType, ok := p.getChannelType(channelID)
	if !ok {
		return nil, nil // unknown channel
	}

	payload := arr[1]
	if payload[0] == '"' {
		ss, err := strconv.Unquote(string(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to parse websocket response: %w", err)
		}

		switch ss {
		case "hb":
			// heartbeat event
			return &HeartBeatEvent{ChannelID: channelID}, nil
		}
	}

	// channel event dispatch
	switch channelType {
	case ChannelTicker:
		return parseTickerEvent(channelID, payload)
	case ChannelBook:
		return parseBookEvent(channelID, payload)
	case ChannelCandles:
		return parseCandleEvent(channelID, payload)
	case ChannelStatus:
		return parseStatusEvent(channelID, payload)
	case ChannelTrades:
		return parseTradeEvent(channelID, payload, arr)
	default:
		return nil, fmt.Errorf("unknown or un-supported channel type: %s, message: %s", channelType, message)
	}
}

// parseTickerEvent parses ticker update or snapshot, including funding ticker events.
// It checks the length of the ticker array to distinguish between trading and funding ticker events.
func parseTickerEvent(channelID int64, payload json.RawMessage) (interface{}, error) {
	var data []json.RawMessage
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("failed to parse ticker event: %w", err)
	}

	// funding ticker event has more than 10 fields (usually 17)
	if len(data) >= 15 {
		evt := &FundingTickerEvent{ChannelID: channelID}
		if err := parseRawArray(data, evt, 1); err != nil {
			return nil, fmt.Errorf("failed to parse funding ticker event: %w", err)
		}
		return evt, nil
	}

	// trading ticker event
	if len(data) >= 10 {
		evt := &TickerEvent{ChannelID: channelID}
		if err := parseRawArray(data, evt, 1); err != nil {
			return nil, fmt.Errorf("failed to parse ticker event: %w", err)
		}

		return evt, nil
	}

	log.Warnf("unknown ticker event length: %d, data: %s", len(data), string(payload))
	return nil, nil
}

// parseBookSnapshotEvent parses trading book entries from a snapshot array.
func parseBookSnapshotEvent(channelID int64, entries [][]json.RawMessage) (*BookSnapshotEvent, error) {
	var snapshot = BookSnapshotEvent{ChannelID: channelID}

	for _, entry := range entries {
		if len(entry) != 3 {
			continue
		}
		be := BookEntry{}
		if err := parseRawArray(entry, &be, 0); err != nil {
			return nil, err
		}

		snapshot.Entries = append(snapshot.Entries, be)
	}
	return &snapshot, nil
}

// parseFundingBookSnapshotEvent parses funding book entries from a snapshot array.
func parseFundingBookSnapshotEvent(channelID int64, entries [][]json.RawMessage) (*FundingBookSnapshotEvent, error) {
	var snapshot = FundingBookSnapshotEvent{ChannelID: channelID}

	for _, entry := range entries {
		if len(entry) != 4 {
			continue
		}
		fbe := FundingBookEntry{}
		if err := parseRawArray(entry, &fbe, 0); err != nil {
			return nil, err
		}
		snapshot.Entries = append(snapshot.Entries, fbe)
	}

	return &snapshot, nil
}

// parseBookEvent parses book update or snapshot.
// It supports both trading and funding book entries.
func parseBookEvent(channelID int64, payload json.RawMessage) (interface{}, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(payload, &arr); err != nil {
		return nil, fmt.Errorf("failed to parse book event: %w", err)
	}

	// snapshot: array of arrays, the internal element is an array of book entries
	if len(arr) > 0 && arr[0][0] == '[' {
		var entries [][]json.RawMessage
		if err := json.Unmarshal(payload, &entries); err != nil {
			return nil, fmt.Errorf("failed to parse book snapshot event: %w", err)
		}

		if len(entries) == 0 {
			return nil, nil
		}

		if len(entries[0]) == 3 {
			return parseBookSnapshotEvent(channelID, entries)
		} else if len(entries[0]) == 4 {
			return parseFundingBookSnapshotEvent(channelID, entries)
		} else {
			return nil, fmt.Errorf("unexpected error, unknown book snapshot entry length: %d, data: %s", len(entries[0]), entries[0])
		}
	}

	// update: single array
	if len(arr) == 3 {
		entry := BookEntry{}
		if err := parseRawArray(arr, &entry, 0); err != nil {
			return nil, fmt.Errorf("failed to parse book update event: %w", err)
		}

		return &BookUpdateEvent{
			ChannelID: channelID,
			Entry:     entry,
		}, nil

	} else if len(arr) == 4 {
		entry := FundingBookEntry{}
		if err := parseRawArray(arr, &entry, 0); err != nil {
			return nil, fmt.Errorf("failed to parse funding book update event: %w", err)
		}
		return &FundingBookUpdateEvent{
			ChannelID: channelID,
			Entry:     entry,
		}, nil
	}

	log.Warnf("unknown book update length: %d, arr: %s", len(arr), arr)
	return nil, nil
}

// parseCandleEvent parses candle update or snapshot.
func parseCandleEvent(channelID int64, payload json.RawMessage) (interface{}, error) {
	var arrData []json.RawMessage
	if err := json.Unmarshal(payload, &arrData); err != nil {
		return nil, err
	}
	if len(arrData) > 0 && arrData[0][0] == '[' {
		// snapshot: array of arrays
		var entries [][]json.RawMessage
		if err := json.Unmarshal(payload, &entries); err != nil {
			return nil, err
		}
		candles := make([]CandleEvent, 0, len(entries))
		for _, entry := range entries {
			ce := CandleEvent{ChannelID: channelID}
			if err := parseRawArray(entry, &ce, 1); err != nil {
				return nil, err
			}
			candles = append(candles, ce)
		}
		return candles, nil
	}
	// update: single array
	ce := &CandleEvent{ChannelID: channelID}
	if err := parseRawArray(arrData, ce, 1); err != nil {
		return nil, err
	}
	return ce, nil
}

// parseStatusEvent parses status update.
func parseStatusEvent(channelID int64, payload json.RawMessage) (interface{}, error) {
	var data []json.RawMessage
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, err
	}

	se := &StatusEvent{ChannelID: channelID}
	if err := parseRawArray(data, se, 1); err != nil {
		return nil, fmt.Errorf("failed to parse status event: %w", err)
	}

	return se, nil
}

// parseTradeEvent parses trade update or snapshot, including trade execution events ("te", "fte") and funding trade snapshot events.
// If arr is provided and arr[1] is "te" or "fte", it parses arr[2] as a trade execution event.
func parseTradeEvent(channelID int64, payload json.RawMessage, arr ...[]json.RawMessage) (interface{}, error) {
	if len(arr) > 0 && len(arr[0]) > 1 {
		var msgType string
		if err := json.Unmarshal(arr[0][1], &msgType); err == nil {
			switch msgType {
			case "te", "fte", "tu", "ftu":
				if len(arr[0]) < 3 {
					return nil, nil
				}

				// parse trade execution event
				var tradeArr []json.RawMessage
				if err := json.Unmarshal(arr[0][2], &tradeArr); err != nil {
					return nil, err
				}
				if (msgType == "te" || msgType == "tu") && len(tradeArr) == 4 {
					te := &MarketTradeEvent{ChannelID: channelID}
					if err := parseRawArray(tradeArr, te, 1); err != nil {
						return nil, fmt.Errorf("failed to parse trade execution event: %w", err)
					}
					return te, nil
				} else if (msgType == "fte" || msgType == "ftu") && len(tradeArr) == 5 {
					fte := &FundingMarketTradeEvent{ChannelID: channelID}
					if err := parseRawArray(tradeArr, fte, 0); err != nil {
						return nil, fmt.Errorf("failed to parse funding trade execution event: %w", err)
					}
					return fte, nil
				} else {
					log.Warnf("unexpected trade execution array length: %d for msgType %s", len(tradeArr), msgType)
					return nil, nil
				}
			}

		}
	}

	// snapshot: array of arrays
	var arrData []json.RawMessage
	if err := json.Unmarshal(payload, &arrData); err != nil {
		log.WithError(err).Errorf("unable to parse trade snapshot: %s", payload)
		return nil, fmt.Errorf("failed to parse trade snapshot: %w", err)
	}

	if len(arrData) > 0 && arrData[0][0] == '[' {
		var entries [][]json.RawMessage
		if err := json.Unmarshal(payload, &entries); err != nil {
			log.WithError(err).Errorf(
				"unable to parse trade snapshot: %s",
				string(payload),
			)

			return nil, fmt.Errorf("failed to parse trade snapshot: %w", err)
		}

		// determine if funding trade snapshot by entry length
		if len(entries) > 0 && len(entries[0]) == 5 {
			fundingTrades := make([]FundingMarketTradeEvent, 0, len(entries))
			for _, entry := range entries {
				fte := FundingMarketTradeEvent{ChannelID: channelID}
				if err := parseRawArray(entry, &fte, 1); err != nil {

					log.WithError(err).Errorf(
						"unable to parse funding trade snapshot entry: %s",
						entry,
					)

					return nil, fmt.Errorf("failed to parse funding trade snapshot event: %w", err)
				}

				fundingTrades = append(fundingTrades, fte)
			}

			return fundingTrades, nil
		}

		// normal market trade snapshot (4 fields)
		trades := make([]MarketTradeEvent, 0, len(entries))
		for _, entry := range entries {
			te := MarketTradeEvent{ChannelID: channelID}
			if err := parseRawArray(entry, &te, 1); err != nil {
				return nil, fmt.Errorf("failed to parse trade snapshot event: %w", err)
			}

			trades = append(trades, te)
		}
		return trades, nil
	}

	// update: single array
	te := &MarketTradeEvent{ChannelID: channelID}
	if err := parseRawArray(arrData, te, 1); err != nil {
		return nil, fmt.Errorf("failed to parse trade update event: %w", err)
	}

	return te, nil
}

// WebSocketAuthRequest represents Bitfinex private websocket authentication request.
type WebSocketAuthRequest struct {
	Event       string   `json:"event"`
	ApiKey      string   `json:"apiKey"`
	AuthSig     string   `json:"authSig"`
	AuthPayload string   `json:"authPayload"`
	AuthNonce   string   `json:"authNonce"`
	Filter      []Filter `json:"filter,omitempty"`
}

// GenerateAuthRequest generates a Bitfinex WebSocketAuthRequest for authentication.
func GenerateAuthRequest(apiKey, apiSecret string, filter ...Filter) WebSocketAuthRequest {
	nonce := strconv.FormatInt(time.Now().Unix(), 10)
	payload := "AUTH" + nonce
	sig := hmac.New(sha512.New384, []byte(apiSecret))
	sig.Write([]byte(payload))
	payloadSign := hex.EncodeToString(sig.Sum(nil))
	return WebSocketAuthRequest{
		Event:       "auth",
		ApiKey:      apiKey,
		AuthSig:     payloadSign,
		AuthPayload: payload,
		AuthNonce:   nonce,
		Filter:      filter,
	}
}

type UserPositionSnapshotEvent struct {
	Positions []UserPosition
}

type UserOrderSnapshotEvent struct {
	Orders []UserOrder
}

// parseUserOrderUpdate parses a single Bitfinex user order array into a UserOrder struct.
// It uses parseRawArray for field mapping.
func parseUserOrderUpdate(arrJson json.RawMessage) (*UserOrder, error) {
	var order UserOrder

	if err := parseJsonArray(arrJson, &order, 0); err != nil {
		return nil, err
	}

	return &order, nil
}

// parseUserOrderSnapshot parses Bitfinex private user order snapshot message.
// It returns a slice of UserOrder objects.
func parseUserOrderSnapshot(arrJson json.RawMessage) (*UserOrderSnapshotEvent, error) {
	var evt UserOrderSnapshotEvent
	var orderArrays []json.RawMessage
	if err := json.Unmarshal(arrJson, &orderArrays); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order snapshot array: %w", err)
	}

	for _, jsonArr := range orderArrays {
		order, err := parseUserOrderUpdate(jsonArr)
		if err != nil {
			log.WithError(err).Warnf("failed to parse order fields: %s", jsonArr)
			continue
		} else if order != nil {
			evt.Orders = append(evt.Orders, *order)
		}
	}

	return &evt, nil
}

// UserPosition represents a Bitfinex user position from private WS API.
type UserPosition struct {
	Symbol            string                      // [0] SYMBOL
	Status            PositionStatus              // [1] STATUS
	Amount            fixedpoint.Value            // [2] AMOUNT
	BasePrice         fixedpoint.Value            // [3] BASE_PRICE
	MarginFunding     fixedpoint.Value            // [4] MARGIN_FUNDING
	MarginFundingType int64                       // [5] MARGIN_FUNDING_TYPE
	PL                *fixedpoint.Value           // [6] PL
	PLPerc            *fixedpoint.Value           // [7] PL_PERC
	PriceLiq          *fixedpoint.Value           // [8] PRICE_LIQ
	Leverage          *fixedpoint.Value           // [9] LEVERAGE
	Flags             *int64                      // [10] FLAGS
	PositionID        int64                       // [11] POSITION_ID
	MtsCreate         *types.MillisecondTimestamp // [12] MTS_CREATE
	MtsUpdate         *types.MillisecondTimestamp // [13] MTS_UPDATE
	_                 any                         // [14] PLACEHOLDER
	Type              int64                       // [15] TYPE
	_                 any                         // [16] PLACEHOLDER
	Collateral        fixedpoint.Value            // [17] COLLATERAL
	CollateralMin     *fixedpoint.Value           // [18] COLLATERAL_MIN
	Meta              map[string]any              // [19] META
}

// parseUserPosition parses a single Bitfinex user position array into a UserPosition struct.
func parseUserPosition(fields json.RawMessage) (*UserPosition, error) {
	var pos UserPosition
	if err := parseJsonArray(fields, &pos, 0); err != nil {
		return nil, err
	}
	return &pos, nil
}

// parseUserPositionSnapshot parses Bitfinex private user position snapshot message.
func parseUserPositionSnapshot(arrJson json.RawMessage) (*UserPositionSnapshotEvent, error) {
	var evt UserPositionSnapshotEvent
	var posArrays []json.RawMessage

	if err := json.Unmarshal(arrJson, &posArrays); err != nil {
		return nil, fmt.Errorf("failed to unmarshal position snapshot array: %w", err)
	}

	for _, fields := range posArrays {
		pos, err := parseUserPosition(fields)
		if err != nil {
			log.WithError(err).Warnf("failed to parse position fields: %s", fields)
			continue
		} else if pos != nil {
			evt.Positions = append(evt.Positions, *pos)
		}
	}

	return &evt, nil
}

// parseUserTrade parses a single Bitfinex user trade array into a UserTrade struct.
func parseUserTrade(fields json.RawMessage) (*UserTrade, error) {
	var trade UserTrade
	if err := parseJsonArray(fields, &trade, 0); err != nil {
		return nil, err
	}
	return &trade, nil
}

// parseWalletSnapshot parses Bitfinex wallet snapshot message into WalletSnapshotEvent.
func parseWalletSnapshot(arrJson json.RawMessage) (*WalletSnapshotEvent, error) {
	var walletArrs []json.RawMessage
	if err := json.Unmarshal(arrJson, &walletArrs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal wallet snapshot array: %w", err)
	}

	wallets := make([]Wallet, 0, len(walletArrs))
	for _, fields := range walletArrs {
		wallet, err := parseWallet(fields)
		if err != nil {
			log.WithError(err).Warnf("failed to parse wallet fields: %s", fields)
			continue
		} else if wallet != nil {
			wallets = append(wallets, *wallet)
		}

	}
	return &WalletSnapshotEvent{
		Wallets: wallets,
	}, nil
}

// parseWallet parses Bitfinex wallet update message into Wallet.
func parseWallet(arrJson json.RawMessage) (*Wallet, error) {
	var wallet Wallet
	if err := parseJsonArray(arrJson, &wallet, 0); err != nil {
		return nil, fmt.Errorf("failed to parse wallet update fields: %w", err)
	}

	return &wallet, nil
}

// UserOrder represents a private user order from Bitfinex API.
// The fields are mapped by array position according to Bitfinex's documentation.
type UserOrder struct {
	OrderID    int64                      // [0] ID
	GID        *int64                     // [1] GID
	CID        *int64                     // [2] CID
	Symbol     string                     // [3] SYMBOL
	CreatedAt  types.MillisecondTimestamp // [4] MTS_CREATE
	UpdatedAt  types.MillisecondTimestamp // [5] MTS_UPDATE
	Amount     fixedpoint.Value           // [6] AMOUNT
	AmountOrig fixedpoint.Value           // [7] AMOUNT_ORIG

	OrderType OrderType // [8] ORDER_TYPE
	TypePrev  *string   // [9] TYPE_PREV

	// MtsTif - Millisecond epoch timestamp for TIF (Time-In-Force)
	MtsTIF *int64 // [10] MTS_TIF

	_      any    // [11] PLACEHOLDER
	Flags  *int64 // [12] FLAGS
	Status string // [13] STATUS

	_ any // [14] PLACEHOLDER
	_ any // [15] PLACEHOLDER

	Price         fixedpoint.Value // [16] PRICE
	PriceAvg      fixedpoint.Value // [17] PRICE_AVG
	PriceTrailing fixedpoint.Value // [18] PRICE_TRAILING
	PriceAuxLimit fixedpoint.Value // [19] PRICE_AUX_LIMIT
	_             any              // [20] PLACEHOLDER
	_             any              // [21] PLACEHOLDER
	_             any              // [22] PLACEHOLDER
	Notify        *int64           // [23] NOTIFY
	Hidden        *int64           // [24] HIDDEN
	PlacedID      *int64           // [25] PLACED_ID
	_             any              // [26] PLACEHOLDER
	_             any              // [27] PLACEHOLDER
	Routing       *string          // [28] ROUTING
	_             any              // [29] PLACEHOLDER
	_             any              // [30] PLACEHOLDER
	Meta          map[string]any   // [31] META (object)
}

// UserTrade represents a Bitfinex user trade from private WS API.
type UserTrade struct {
	ID         int64                      // [0] TRADE ID
	Symbol     string                     // [1] SYMBOL
	Time       types.MillisecondTimestamp // [2] MTS_CREATE
	OrderID    int64                      // [3] ORDER_ID
	ExecAmount fixedpoint.Value           // [4] EXEC_AMOUNT
	ExecPrice  fixedpoint.Value           // [5] EXEC_PRICE

	OrderType  string           // [6] ORDER_TYPE
	OrderPrice fixedpoint.Value // [7] ORDER_PRICE

	// Maker field: 1 if true, -1 if false
	Maker int // [8] MAKER

	Fee         *fixedpoint.Value // [9] FEE (nullable, only for 'tu')
	FeeCurrency *string           // [10] FEE_CURRENCY (nullable, only for 'tu')

	ClientOrderID int64 // [11] ClientOrderID (Client Order ID)
}

// BalanceUpdateEvent represents a Bitfinex balance update event.
type BalanceUpdateEvent struct {
	AUM    fixedpoint.Value
	AUMNet fixedpoint.Value
}

// parseBalanceUpdateEvent parses Bitfinex balance update event.
func parseBalanceUpdateEvent(payload json.RawMessage) (interface{}, error) {
	var event BalanceUpdateEvent
	if err := parseJsonArray(payload, &event, 0); err != nil {
		return nil, fmt.Errorf("failed to parse balance update event: %w", err)
	}

	return &event, nil
}

// FundingOfferUpdateEvent represents a Bitfinex funding offer update event.
type FundingOfferUpdateEvent struct {
	OfferID    int64                      // [0] OFFER_ID
	Symbol     string                     // [1] SYMBOL
	MtsCreated types.MillisecondTimestamp // [2] MTS_CREATED
	MtsUpdated types.MillisecondTimestamp // [3] MTS_UPDATED
	Amount     fixedpoint.Value           // [4] AMOUNT
	AmountOrig fixedpoint.Value           // [5] AMOUNT_ORIG
	OfferType  string                     // [6] OFFER_TYPE
	_          any                        // [7] PLACEHOLDER
	_          any                        // [8] PLACEHOLDER
	Flags      int64                      // [9] FLAGS
	Status     string                     // [10] STATUS
	_          any                        // [11] PLACEHOLDER
	_          any                        // [12] PLACEHOLDER
	_          any                        // [13] PLACEHOLDER
	Rate       fixedpoint.Value           // [14] RATE
	Period     int64                      // [15] PERIOD
	Notify     int64                      // [16] NOTIFY
	Hidden     int64                      // [17] HIDDEN
	_          any                        // [18] PLACEHOLDER
	Renew      int64                      // [19] RENEW
	RateReal   any                        // [20] RATE_REAL (nullable)
}

// parseFundingOfferUpdate parses a single funding offer update array into FundingOfferUpdateEvent.
func parseFundingOfferUpdate(arrJson json.RawMessage) (*FundingOfferUpdateEvent, error) {
	var event FundingOfferUpdateEvent
	if err := parseJsonArray(arrJson, &event, 0); err != nil {
		return nil, fmt.Errorf("failed to parse funding offer update: %w", err)
	}

	return &event, nil
}

// parseFundingOfferSnapshot parses a funding offer snapshot array into []FundingOfferUpdateEvent.
func parseFundingOfferSnapshot(arrJson json.RawMessage) ([]FundingOfferUpdateEvent, error) {
	var offerArrays []json.RawMessage
	if err := json.Unmarshal(arrJson, &offerArrays); err != nil {
		return nil, fmt.Errorf("failed to unmarshal funding offer snapshot array: %w", err)
	}

	events := make([]FundingOfferUpdateEvent, 0, len(offerArrays))
	for _, jsonArr := range offerArrays {
		event, err := parseFundingOfferUpdate(jsonArr)
		if err != nil {
			log.WithError(err).Warnf("failed to parse funding offer fields: %s", jsonArr)
			continue
		} else if event != nil {
			events = append(events, *event)
		}
	}

	return events, nil
}

// FundingInfo represents the inner funding info array.
type FundingInfo struct {
	YieldLoan    fixedpoint.Value // [0] YIELD_LOAN
	YieldLend    fixedpoint.Value // [1] YIELD_LEND
	DurationLoan fixedpoint.Value // [2] DURATION_LOAN
	DurationLend fixedpoint.Value // [3] DURATION_LEND
}

func (i *FundingInfo) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, i, 0)
}

// FundingInfoEvent represents the wrapper for Bitfinex funding info update event.
type FundingInfoEvent struct {
	UpdateType string      // [0] UPDATE_TYPE (e.g. "sym")
	Symbol     string      // [1] SYMBOL (e.g. "fUSD")
	Info       FundingInfo // [2] FundingInfo array
}

// parseFundingInfoEvent parses Bitfinex funding info update message into FundingInfoEvent.
func parseFundingInfoEvent(arrJson json.RawMessage) (*FundingInfoEvent, error) {
	var event FundingInfoEvent

	if err := parseJsonArray(arrJson, &event, 0); err != nil {
		return nil, fmt.Errorf("failed to parse funding info event: %w", err)
	}

	return &event, nil
}
