package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/types"
)

// The futures user data stream (fstream) only pushes ACCOUNT_UPDATE when a balance or a position
// changes, so it never reports risk figures such as the maintenance margin or the margin balance
// as mark prices move. The futures WebSocket API (ws-fapi) can return the full account status on
// request, so when enabled, a dedicated ws-fapi connection sends a signed v2/account.status request
// on every interval and emits the response as a FuturesAccountStatusEvent.
//
// See https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/ws-api/account

const (
	futuresAccountStatusMethod = "v2/account.status"

	// v2/account.status costs 5 request weight per call.
	DefaultFuturesAccountStatusInterval = 10 * time.Second
	minFuturesAccountStatusInterval     = time.Second
)

// FuturesAccountStatusEvent is the response of the futures WebSocket API method v2/account.status.
//
// Sample response:
//
//	{"id":"account-status-1","status":200,"result":{"totalInitialMargin":"0.00000000",
//	  "totalMaintMargin":"0.00000000","totalWalletBalance":"103.12345678","totalMarginBalance":"103.12345678",
//	  ..., "assets":[...], "positions":[...]},
//	  "rateLimits":[{"rateLimitType":"REQUEST_WEIGHT","interval":"MINUTE","intervalNum":1,"limit":2400,"count":10}]}
type FuturesAccountStatusEvent struct {
	ID         string                    `json:"id"`
	Status     int                       `json:"status"`
	Account    binanceapi.FuturesAccount `json:"result"`
	RateLimits []RateLimit               `json:"rateLimits,omitempty"`

	// ReceivedAt is the local time when the response was received
	ReceivedAt time.Time `json:"-"`
}

// FuturesAccount converts the account status into the global futures account type.
func (e *FuturesAccountStatusEvent) FuturesAccount() *types.FuturesAccount {
	return toGlobalFuturesAccountInfo(&e.Account, nil)
}

type futuresWsApiErrorEvent struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
	Error  *Error `json:"error"`
}

type futuresWsApiSignedParams struct {
	APIKey    string `json:"apiKey"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
}

// EnableFuturesAccountStatusUpdate makes a futures user data stream poll the account status
// (v2/account.status) over the futures WebSocket API every interval and emit it via
// OnFuturesAccountStatusEvent. A non-positive interval uses DefaultFuturesAccountStatusInterval.
// It must be called before Connect, and only takes effect on a futures user data stream.
func (s *Stream) EnableFuturesAccountStatusUpdate(interval time.Duration) {
	if interval <= 0 {
		interval = DefaultFuturesAccountStatusInterval
	} else if interval < minFuturesAccountStatusInterval {
		interval = minFuturesAccountStatusInterval
	}

	s.futuresAccountStatusInterval = interval
}

func (s *Stream) shouldUseFuturesAccountStatusStream() bool {
	return s.exchange.IsFutures && !s.PublicOnly && s.futuresAccountStatusInterval > 0
}

func (s *Stream) initFuturesAccountStatusStream() *types.StandardStream {
	aux := types.NewStandardStream()
	aux.SetParser(parseFuturesWsApiResponse)
	aux.SetDispatcher(s.dispatchFuturesWsApiEvent)
	aux.SetEndpointCreator(func(ctx context.Context) (string, error) {
		if testNet {
			return WsTestNetFuturesWebSocketURL, nil
		}
		return WsFuturesWebSocketURL, nil
	})

	// the heartbeat runs on the ping worker right before each ping frame, so requests are never
	// written concurrently with pings; the ping interval doubles as the polling interval.
	aux.SetPingInterval(s.futuresAccountStatusInterval)
	aux.SetHeartBeat(s.writeFuturesAccountStatusRequest)
	aux.OnConnect(func() {
		// OnConnect is emitted before the read and ping workers start, so writing here is safe
		if err := s.writeFuturesAccountStatusRequest(aux.Conn); err != nil {
			log.WithError(err).Error("futures account status request error")
		}
	})

	return &aux
}

// connectFuturesAccountStatusStream connects the account status stream with the base context of
// the user data stream. The stream reconnects on its own once connected; the initial connect is
// retried here so that a ws-fapi failure never blocks the user data stream.
func (s *Stream) connectFuturesAccountStatusStream(ctx context.Context, aux *types.StandardStream) {
	for {
		err := aux.Connect(ctx)
		if err == nil {
			return
		}

		log.WithError(err).Warn("futures account status stream connect error, retrying...")
		select {
		case <-ctx.Done():
			return
		case <-aux.CloseC:
			return
		case <-time.After(15 * time.Second):
		}
	}
}

func (s *Stream) writeFuturesAccountStatusRequest(conn *websocket.Conn) error {
	params, err := s.signFuturesWsApiParams()
	if err != nil {
		return err
	}

	id := atomic.AddUint64(&s.futuresWsApiRequestID, 1)
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}

	return conn.WriteJSON(&WebSocketCommand{
		ID:     "account-status-" + strconv.FormatUint(id, 10),
		Method: futuresAccountStatusMethod,
		Params: params,
	})
}

// signFuturesWsApiParams signs the request parameters the same way the REST API does:
// the parameters sorted by key and joined as a query string, signed with HMAC-SHA256
// or with the ed25519 private key when no secret is configured.
func (s *Stream) signFuturesWsApiParams() (*futuresWsApiSignedParams, error) {
	timestamp := time.Now().UnixMilli() - s.exchange.futuresClient2.TimeOffset()
	payload := url.Values{}
	payload.Add("apiKey", s.exchange.key)
	payload.Add("timestamp", strconv.FormatInt(timestamp, 10))
	toSign := payload.Encode()

	var signature string
	switch {
	case len(s.exchange.secret) > 0:
		mac := hmac.New(sha256.New, []byte(s.exchange.secret))
		if _, err := mac.Write([]byte(toSign)); err != nil {
			return nil, err
		}
		signature = hex.EncodeToString(mac.Sum(nil))

	case len(s.ed25519authentication.privateKey) > 0:
		signature = binanceapi.GenerateSignatureEd25519(toSign, s.ed25519authentication.privateKey)

	default:
		return nil, fmt.Errorf("binance futures websocket api: api secret or ed25519 private key is required")
	}

	return &futuresWsApiSignedParams{
		APIKey:    s.exchange.key,
		Timestamp: timestamp,
		Signature: signature,
	}, nil
}

func parseFuturesWsApiResponse(message []byte) (interface{}, error) {
	var header struct {
		ID     string          `json:"id"`
		Status int             `json:"status"`
		Error  json.RawMessage `json:"error"`
	}

	if err := json.Unmarshal(message, &header); err != nil {
		return nil, err
	}

	if header.Status != 200 || len(header.Error) > 0 {
		var event futuresWsApiErrorEvent
		err := json.Unmarshal(message, &event)
		return &event, err
	}

	var event FuturesAccountStatusEvent
	if err := json.Unmarshal(message, &event); err != nil {
		return nil, err
	}

	event.ReceivedAt = time.Now()
	return &event, nil
}

func (s *Stream) dispatchFuturesWsApiEvent(e interface{}) {
	switch e := e.(type) {
	case *FuturesAccountStatusEvent:
		s.EmitFuturesAccountStatusEvent(e)

	case *futuresWsApiErrorEvent:
		log.Errorf("futures websocket api error response: id=%s status=%d error=%+v", e.ID, e.Status, e.Error)
	}
}
