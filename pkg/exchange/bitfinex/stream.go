package bitfinex

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/depth"
	bfxapi "github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/types"
)

// Stream represents the Bitfinex websocket stream.
//
//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	depthBuffers map[string]*depth.Buffer

	tickerEventCallbacks      []func(e *bfxapi.TickerEvent)
	bookEventCallbacks        []func(e *bfxapi.BookEvent)
	candleEventCallbacks      []func(e *bfxapi.CandleEvent)
	statusEventCallbacks      []func(e *bfxapi.StatusEvent)
	marketTradeEventCallbacks []func(e *bfxapi.MarketTradeEvent)

	parser *bfxapi.Parser

	ex *Exchange
}

// NewStream creates a new Bitfinex Stream.
func NewStream(ex *Exchange) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		depthBuffers:   make(map[string]*depth.Buffer),
		parser:         bfxapi.NewParser(),
		ex:             ex,
	}
	stream.SetParser(stream.parser.Parse)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetEndpointCreator(stream.getEndpoint)
	stream.OnConnect(stream.onConnect)
	return stream
}

// getEndpoint returns the websocket endpoint URL.
func (s *Stream) getEndpoint(ctx context.Context) (string, error) {
	url := os.Getenv("BITFINEX_API_WS_URL")
	if url == "" {
		if s.PublicOnly {
			url = bfxapi.PublicWebSocketURL
		} else {
			url = bfxapi.PrivateWebSocketURL
		}
	}
	return url, nil
}

// onConnect handles authentication for private websocket endpoint.
func (s *Stream) onConnect() {
	ctx := context.Background()
	endpoint, err := s.getEndpoint(ctx)
	if err != nil {
		logrus.WithError(err).Error("bitfinex websocket: failed to get endpoint")
		return
	}

	if endpoint == bfxapi.PrivateWebSocketURL {
		apiKey := s.ex.apiKey
		apiSecret := s.ex.apiSecret
		if apiKey == "" || apiSecret == "" {
			logrus.Warn("bitfinex private websocket: missing API key or secret")
		}

		nonce := fmt.Sprintf("%v", time.Now().Unix())
		payload := "AUTH" + nonce
		sig := hmac.New(sha512.New384, []byte(apiSecret))
		sig.Write([]byte(payload))
		payloadSign := hex.EncodeToString(sig.Sum(nil))
		authMsg := WebSocketAuthRequest{
			Event:       "auth",
			ApiKey:      apiKey,
			AuthSig:     payloadSign,
			AuthPayload: payload,
			AuthNonce:   nonce,
		}

		if err := s.Conn.WriteJSON(authMsg); err != nil {
			logrus.WithError(err).Error("bitfinex auth: failed to send auth message")
			return
		}

		logrus.Info("bitfinex private websocket: sent auth message")
	}
}

// dispatchEvent dispatches parsed events to corresponding callbacks.
func (s *Stream) dispatchEvent(e interface{}) {
	switch evt := e.(type) {
	default:
		logrus.Warnf("unhandled %T event: %+v", evt, evt)
	}
}

// WebSocketAuthRequest represents Bitfinex private websocket authentication request.
type WebSocketAuthRequest struct {
	Event       string `json:"event"`
	ApiKey      string `json:"apiKey"`
	AuthSig     string `json:"authSig"`
	AuthPayload string `json:"authPayload"`
	AuthNonce   string `json:"authNonce"`
}
