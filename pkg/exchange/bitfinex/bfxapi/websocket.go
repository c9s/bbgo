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

// BookEvent represents an order book update or snapshot event.
type BookEvent struct {
	ChannelID int64

	// Trading book
	Price  fixedpoint.Value
	Count  int64
	Amount fixedpoint.Value
}

// FundingBookEvent represents a funding book entry event.
type FundingBookEvent struct {
	ChannelID int64

	Rate   fixedpoint.Value
	Period int64
	Count  int64
	Amount fixedpoint.Value
}

// CandleEvent represents a kline/candle update or snapshot event.
type CandleEvent struct {
	ChannelID int64
	Symbol    string
	MTS       int64
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

	logrus.Warnf("unknown websocket message format: %s", string(message))
	return nil, nil
}

// parseObjectMessage parses a JSON object message (WebSocketResponse).
func (p *Parser) parseObjectMessage(message []byte) (interface{}, error) {
	var resp WebSocketResponse
	if err := json.Unmarshal(message, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse websocket response: %w", err)
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

	logrus.Warnf("unknown ticker event length: %d, data: %s", len(data), string(payload))
	return nil, nil
}

// parseBookEvent parses book update or snapshot.
// It supports both trading and funding book entries.
func parseBookEvent(channelID int64, payload json.RawMessage) (interface{}, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(payload, &arr); err != nil {
		return nil, fmt.Errorf("failed to parse book event: %w", err)
	}

	// snapshot: array of arrays
	if len(arr) > 0 && arr[0][0] == '[' {
		var entries [][]json.RawMessage
		if err := json.Unmarshal(payload, &entries); err != nil {
			return nil, fmt.Errorf("failed to parse book snapshot event: %w", err)
		}
		var books []BookEvent
		var fundingBooks []FundingBookEvent
		for _, entry := range entries {
			if len(entry) == 3 {
				be := BookEvent{ChannelID: channelID}
				if err := parseRawArray(entry, &be, 1); err != nil {
					return nil, fmt.Errorf("failed to parse book snapshot event: %w", err)
				}

				books = append(books, be)
			} else if len(entry) == 4 {
				fbe := FundingBookEvent{ChannelID: channelID}
				if err := parseRawArray(entry, &fbe, 1); err != nil {
					return nil, fmt.Errorf("failed to parse book snapshot event: %w", err)
				}

				fundingBooks = append(fundingBooks, fbe)
			} else {
				logrus.Errorf("unknown book snapshot entry length: %d, entry: %s", len(entry), entry)
				continue
			}
		}
		if len(fundingBooks) > 0 {
			return fundingBooks, nil
		}
		return books, nil
	}

	// update: single array
	if len(arr) == 3 {
		be := &BookEvent{ChannelID: channelID}
		if err := parseRawArray(arr, be, 1); err != nil {
			return nil, fmt.Errorf("failed to parse book update event: %w", err)
		}
		return be, nil
	} else if len(arr) == 4 {
		fbe := &FundingBookEvent{ChannelID: channelID}
		if err := parseRawArray(arr, fbe, 1); err != nil {
			return nil, fmt.Errorf("failed to parse funding book update event: %w", err)
		}
		return fbe, nil
	} else {
		logrus.Warnf("unknown book update length: %d, arr: %s", len(arr), arr)
		return nil, nil
	}
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
					logrus.Warnf("unexpected trade execution array length: %d for msgType %s", len(tradeArr), msgType)
					return nil, nil
				}
			}

		}
	}

	// snapshot: array of arrays
	var arrData []json.RawMessage
	if err := json.Unmarshal(payload, &arrData); err != nil {
		return nil, fmt.Errorf("failed to parse trade snapshot: %w", err)
	}

	if len(arrData) > 0 && arrData[0][0] == '[' {
		var entries [][]json.RawMessage
		if err := json.Unmarshal(payload, &entries); err != nil {
			return nil, fmt.Errorf("failed to parse trade snapshot: %w", err)
		}

		// determine if funding trade snapshot by entry length
		if len(entries) > 0 && len(entries[0]) == 5 {
			fundingTrades := make([]FundingMarketTradeEvent, 0, len(entries))
			for _, entry := range entries {
				fte := FundingMarketTradeEvent{ChannelID: channelID}
				if err := parseRawArray(entry, &fte, 1); err != nil {
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
		return nil, err
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
