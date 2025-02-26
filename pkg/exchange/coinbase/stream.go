package coinbase

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

const wsFeedURL = "wss://ws-feed.exchange.coinbase.com"

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream
	apiKey     string
	passphrase string
	secretKey  string

	// callbacks
	statusMessageCallbacks   []func(m *StatusMessage)
	auctionMessageCallbacks  []func(m *AuctionMessage)
	rfqMessageCallbacks      []func(m *RfqMessage)
	tickerMessageCallbacks   []func(m *TickerMessage)
	receivedMessageCallbacks []func(m *ReceivedMessage)
	openMessageCallbacks     []func(m *OpenMessage)
	doneMessageCallbacks     []func(m *DoneMessage)
	matchMessageCallbacks    []func(m *MatchMessage)
	changeMessageCallbacks   []func(m *ChangeMessage)
	activeMessageCallbacks   []func(m *ActiveMessage)
}

func NewStream(
	apiKey string,
	passphrase string,
	secretKey string,
) *Stream {
	s := Stream{
		StandardStream: types.NewStandardStream(),
		apiKey:         apiKey,
		passphrase:     passphrase,
		secretKey:      secretKey,
	}
	s.SetParser(s.parseMessage)
	s.SetDispatcher(s.dispatchEvent)
	s.SetEndpointCreator(createEndpoint)

	// public handlers
	s.OnConnect(s.handleConnect)

	return &s
}

func (s *Stream) dispatchEvent(e interface{}) {
	switch e := e.(type) {
	case *StatusMessage:
		s.EmitStatusMessage(e)
	case *AuctionMessage:
		s.EmitAuctionMessage(e)
	case *RfqMessage:
		s.EmitRfqMessage(e)
	case *TickerMessage:
		s.EmitTickerMessage(e)
	case *ReceivedLimitOrderMessage:
		s.EmitReceivedLimitOrderMessage(e)
	case *ReceivedMarketOrderMessage:
		s.EmitReceivedMarketOrderMessage(e)
	case *OpenMessage:
		s.EmitOpenMessage(e)
	case *DoneMessage:
		s.EmitDoneMessage(e)
	case *MatchMessage:
		s.EmitMatchMessage(e)
	case *AuthMakerMatchMessage:
		s.EmitAuthMakerMatchMessage(e)
	case *AuthTakerMatchMessage:
		s.EmitAuthTakerMatchMessage(e)
	case *StpChangeMessage:
		s.EmitStpChangeMessage(e)
	case *ModifyOrderChangeMessage:
		s.EmitModifyOrderChangeMessage(e)
	case *ActiveMessage:
		s.EmitActiveMessage(e)
	default:
		log.Warnf("skip dispatching msg due to unknown message type: %T", e)
	}
}

func createEndpoint(ctx context.Context) (string, error) {
	return wsFeedURL, nil
}

type channelType struct {
	Name       string   `json:"name"`
	ProductIDs []string `json:"product_ids,omitempty"`
}

type websocketCommand struct {
	Type       string        `json:"type"`
	Channels   []channelType `json:"channels"`
	Signature  *string       `json:"signature,omitempty"`
	Key        *string       `json:"key,omitempty"`
	Passphrase *string       `json:"passphrase,omitempty"`
	Timestamp  *string       `json:"timestamp,omitempty"`
}

func (s *Stream) handleConnect() {
	// subscribe to channels
	if len(s.Subscriptions) == 0 {
		return
	}

	subProductsMap := make(map[string][]string)
	for _, sub := range s.Subscriptions {
		strChannel := string(sub.Channel)
		if _, ok := subProductsMap[strChannel]; !ok {
			subProductsMap[strChannel] = []string{}
		}
		// "rfq_matches" allow empty symbol
		if sub.Channel != "rfq_matches" && len(sub.Symbol) == 0 {
			continue
		}
		products := subProductsMap[strChannel]
		products = append(products, sub.Symbol)
		subProductsMap[strChannel] = products
	}
	subCmds := []websocketCommand{}
	signature, ts := s.generateSignature()
	authEnabled := !s.PublicOnly && len(s.apiKey) > 0 && len(s.passphrase) > 0 && len(s.secretKey) > 0
	for channel, productIDs := range subProductsMap {
		var subType string
		switch channel {
		case "rfq_matches":
			subType = "subscriptions"
		default:
			subType = "subscribe"
		}
		subCmd := websocketCommand{
			Type: subType,
			Channels: []channelType{
				{
					Name:       channel,
					ProductIDs: productIDs,
				},
			},
		}
		if authEnabled {
			subCmd.Signature = &signature
			subCmd.Key = &s.apiKey
			subCmd.Passphrase = &s.passphrase
			subCmd.Timestamp = &ts
		}
		subCmds = append(subCmds, subCmd)
	}
	for _, subCmd := range subCmds {
		err := s.Conn.WriteJSON(subCmd)
		if err != nil {
			log.WithError(err).Errorf("subscription error: %v", subCmd)
		}
	}
}

func (s *Stream) generateSignature() (string, string) {
	// Convert current time to string timestamp
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	// Create message string
	message := ts + "GET/users/self/verify"

	// Decode base64 secret
	secretBytes, err := base64.StdEncoding.DecodeString(s.secretKey)
	if err != nil {
		log.WithError(err).Error("failed to decode secret key")
		return "", ""
	}

	// Create HMAC-SHA256
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(message))

	// Get signature and encode to base64
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature, ts
}
