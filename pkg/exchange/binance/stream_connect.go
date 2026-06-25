package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"time"

	api "github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/gorilla/websocket"
)

type UserDataStreamType string

const (
	UserDataStreamUnknown        UserDataStreamType = "unknown"
	UserDataStreamEd25519Auth    UserDataStreamType = "ed25519_auth"
	UserDataStreamSpot           UserDataStreamType = "spot"
	UserDataStreamMargin         UserDataStreamType = "margin"
	UserDataStreamIsolatedMargin UserDataStreamType = "isolated_margin"
	UserDataStreamFutures        UserDataStreamType = "futures"
)

func (s *Stream) handleConnect() {
	if s.PublicOnly {
		// market data stream
		if s.exchange.IsFutures {
			// Determine which subscriptions to send on the main connection:
			//   - mixed or market-only → main conn is /market → send futuresMarketSubs
			//   - public-only (depth/bookTicker only) → main conn is /public → send futuresPublicSubs
			mainSubs := s.futuresMarketSubs
			if s.futuresAuxStream == nil && len(s.futuresPublicSubs) > 0 {
				mainSubs = s.futuresPublicSubs
			}
			if err := s.writeSpecificSubscriptions(s.Conn, mainSubs); err != nil {
				log.WithError(err).Error("futures subscribe error")
			}
			// if mixed subscriptions, connect aux stream to /public independently
			if s.futuresAuxStream != nil {
				connCtx := s.ConnCtx
				go func() {
					if err := s.futuresAuxStream.DialAndConnect(connCtx); err != nil {
						log.WithError(err).Error("futures aux stream (/public) connect error")
					}
				}()
			}
		} else {
			if err := s.writeSubscriptions(); err != nil {
				log.WithError(err).Error("subscribe error")
			}
		}
	} else {
		// user data stream
		userStreamType := s.detectUserDataStreamType()
		switch userStreamType {
		case UserDataStreamEd25519Auth:
			if err := s.sendEd25519LoginCommand(); err != nil {
				log.WithError(err).Error("ed25519 auth error")
			}

			time.Sleep(1 * time.Second)

			// futures does not support ed25519 login on WsApi
			if !s.exchange.IsFutures {
				if err := s.sendSubscribeUserDataStreamCommand(); err != nil {
					log.WithError(err).Error("subscribe user data stream error")
				}
			}

			// TODO: ensure that we receive an authorized event to trigger this auth event
			go s.EmitAuth()
		case UserDataStreamFutures:
			go s.EmitAuth()
		case UserDataStreamMargin, UserDataStreamIsolatedMargin:
			// Skip subscription if listenToken is missing or expired
			if len(s.listenToken) == 0 || time.Now().After(s.listenTokenExpiration) {
				return
			}

			// Use new listenToken subscription method (recommended as of 2025-10-06)
			// This still uses the WebSocket API endpoint but with listenToken instead of ed25519 auth
			if err := s.sendListenTokenSubscribeCommand(); err != nil {
				log.WithError(err).Error("listenToken subscribe error")
			}

			go s.EmitAuth()
		case UserDataStreamSpot:
			// spot trading
			if err := s.sendSpotHmacUserDataStreamCommand(); err != nil {
				log.WithError(err).Error("spot hmac user data stream subscribe error")
			}

			go s.EmitAuth()
		default:
			// Emit Auth before establishing the connection to prevent the caller from missing the Update data after
			// creating the order.
			// spawn a goroutine to emit auth event to prevent blocking the main event loop
			log.Warnf("unknown user data stream type: %s", userStreamType)
			go s.EmitAuth()
		}
	}
}

func (s *Stream) detectUserDataStreamType() UserDataStreamType {
	if s.canUseWsApiEndpoint() {
		return UserDataStreamEd25519Auth
	}
	if s.exchange.IsFutures {
		return UserDataStreamFutures
	}
	if !s.exchange.useListenKey {
		if s.exchange.IsMargin {
			return UserDataStreamMargin
		}
		if s.exchange.IsIsolatedMargin {
			return UserDataStreamIsolatedMargin
		}
		return UserDataStreamSpot
	}
	return UserDataStreamUnknown
}

// writeSpecificSubscriptions sends a SUBSCRIBE command for a given set of subscriptions
// on the provided connection. Returns nil without writing if subs is empty or conn is nil.
func (s *Stream) writeSpecificSubscriptions(conn *websocket.Conn, subs []types.Subscription) error {
	if len(subs) == 0 {
		return nil
	}
	var params []string
	for _, subscription := range subs {
		params = append(params, convertSubscription(subscription))
	}
	// TODO: handle subscribe response
	// sample response: {"result":null,"id":2}
	log.Infof("subscribing channels: %+v", params)
	return conn.WriteJSON(&WebSocketCommand{
		Method: "SUBSCRIBE",
		Params: params,
		ID:     2,
	})
}

// canUseWsApiEndpoint returns true if the stream should use ed25519 authentication with the new ws api endpoint
// only the new spot trading websocket endpoint supports ed25519 authentication
func (s *Stream) canUseWsApiEndpoint() bool {
	// margin and futures don't support ed25519 authentication
	if s.exchange.MarginSettings.IsMargin || s.exchange.FuturesSettings.IsFutures {
		return false
	}

	return s.ed25519authentication.usingEd25519
}

// sendEd25519LoginCommand
// Sample response:
//
//	  {"id":1,"status":200,"result":{"apiKey":".....",
//		  "authorizedSince":1746456891079,
//		  "connectedSince":1746456891107,
//		  "returnRateLimits":true,
//		  "serverTime":1746456891153,"userDataStream":false},
//		"rateLimits":[{"rateLimitType":"REQUEST_WEIGHT","interval":"MINUTE","intervalNum":1,"limit":6000,"count":14}]}
func (s *Stream) sendEd25519LoginCommand() error {
	timestamp := time.Now().UnixMilli()
	params := url.Values{}
	params.Add("apiKey", s.client.APIKey)
	params.Add("timestamp", strconv.FormatInt(timestamp, 10))
	paramsStr := params.Encode()
	signature := api.GenerateSignatureEd25519(paramsStr, s.ed25519authentication.privateKey)
	return s.Conn.WriteJSON(&WebSocketCommand{
		Method: "session.logon",
		ID:     1,
		Params: &LogonParams{
			APIKey:    s.client.APIKey,
			Signature: signature,
			Timestamp: timestamp,
		},
	})
}

func (s *Stream) sendSubscribeUserDataStreamCommand() error {
	return s.Conn.WriteJSON(&WebSocketCommand{
		ID:     2,
		Method: "userDataStream.subscribe",
	})
}

// sendListenTokenSubscribeCommand sends the new listenToken subscription command (2025-10-06 recommended)
func (s *Stream) sendListenTokenSubscribeCommand() error {
	return s.Conn.WriteJSON(&WebSocketCommand{
		ID:     2,
		Method: "userDataStream.subscribe.listenToken",
		Params: ListenTokenSubscribeParams{
			ListenToken: s.listenToken,
		},
	})
}

type SpotHmacSubscribeParams struct {
	Key       string `json:"apiKey"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature,omitempty"`
}

func (s *Stream) sendSpotHmacUserDataStreamCommand() error {
	mac := hmac.New(
		sha256.New, []byte(s.exchange.secret),
	)

	timestamp := time.Now().UnixMilli()
	payload := url.Values{}
	payload.Add("apiKey", s.exchange.key)
	payload.Add("timestamp", strconv.FormatInt(timestamp, 10))
	payloadBytes := []byte(payload.Encode())
	if _, err := mac.Write(payloadBytes); err != nil {
		return err
	}
	signature := hex.EncodeToString(mac.Sum(nil))

	return s.Conn.WriteJSON(&WebSocketCommand{
		ID:     2,
		Method: "userDataStream.subscribe.signature",
		Params: SpotHmacSubscribeParams{
			Key:       s.exchange.key,
			Timestamp: timestamp,
			Signature: signature,
		},
	})
}
