package coinbase

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/types"
)

const wsFeedURL = "wss://ws-feed.exchange.coinbase.com"
const rfqMatchChannel = "rfq_matches"

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	exchange   *Exchange
	apiKey     string
	passphrase string
	secretKey  string

	// callbacks
	statusMessageCallbacks            []func(m *StatusMessage)
	auctionMessageCallbacks           []func(m *AuctionMessage)
	rfqMessageCallbacks               []func(m *RfqMessage)
	tickerMessageCallbacks            []func(m *TickerMessage)
	receivedMessageCallbacks          []func(m *ReceivedMessage)
	openMessageCallbacks              []func(m *OpenMessage)
	doneMessageCallbacks              []func(m *DoneMessage)
	matchMessageCallbacks             []func(m *MatchMessage)
	changeMessageCallbacks            []func(m *ChangeMessage)
	activeMessageCallbacks            []func(m *ActiveMessage)
	balanceMessageCallbacks           []func(m *BalanceMessage)
	orderbookSnapshotMessageCallbacks []func(m *OrderBookSnapshotMessage)
	orderbookUpdateMessageCallbacks   []func(m *OrderBookUpdateMessage)

	lock               sync.Mutex // lock to protect lastSequenceMsgMap
	lastSequenceMsgMap map[MessageType]SequenceNumberType
}

func NewStream(
	exchange *Exchange,
	apiKey string,
	passphrase string,
	secretKey string,
) *Stream {
	s := Stream{
		StandardStream: types.NewStandardStream(),
		exchange:       exchange,
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
	case *ReceivedMessage:
		s.EmitReceivedMessage(e)
	case *OpenMessage:
		s.EmitOpenMessage(e)
	case *DoneMessage:
		s.EmitDoneMessage(e)
	case *MatchMessage:
		s.EmitMatchMessage(e)
	case *ChangeMessage:
		s.EmitChangeMessage(e)
	case *ActiveMessage:
		s.EmitActiveMessage(e)
	case *BalanceMessage:
		s.EmitBalanceMessage(e)
	case *OrderBookSnapshotMessage:
		s.EmitOrderbookSnapshotMessage(e)
	case *OrderBookUpdateMessage:
		s.EmitOrderbookUpdateMessage(e)
	default:
		log.Warnf("skip dispatching msg due to unknown message type: %T", e)
	}
}

func createEndpoint(ctx context.Context) (string, error) {
	return wsFeedURL, nil
}

func (s *Stream) AuthEnabled() bool {
	return !s.PublicOnly && len(s.apiKey) > 0 && len(s.passphrase) > 0 && len(s.secretKey) > 0
}

func (s *Stream) generateSignature() (string, string) {
	if len(s.apiKey) == 0 || len(s.passphrase) == 0 || len(s.secretKey) == 0 {
		return "", ""
	}
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

func (s *Stream) handleConnect() {
	// TODO: dummy, will add connection logic later
	return
}
