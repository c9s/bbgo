package coinbase

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/core/klinedriver"
	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// https://docs.cdp.coinbase.com/exchange/docs/websocket-overview
const wsFeedUrl = "wss://ws-feed.exchange.coinbase.com" // ws feeds available without auth

var (
	// interface implementations compile-time check
	_ types.PrivateChannelSymbolSetter = (*Stream)(nil)
	_ types.Stream                     = (*Stream)(nil)
)

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	exchange   *Exchange
	apiKey     string
	passphrase string
	secretKey  string

	logger logrus.FieldLogger

	// channel2LocalIdsMap is a map from channel to local ids, including symbols and other ids
	channel2LocalIdsMap map[types.Channel][]string

	// callbacks
	errorMessageCallbacks             []func(m *ErrorMessage)
	subscriptionsCallbacks            []func(m *SubscriptionsMessage)
	statusMessageCallbacks            []func(m *StatusMessage)
	auctionMessageCallbacks           []func(m *AuctionMessage)
	rfqMessageCallbacks               []func(m *RfqMessage)
	tickerMessageCallbacks            []func(m *TickerMessage)
	receivedMessageCallbacks          []func(m *ReceivedMessage)
	openMessageCallbacks              []func(m *OpenMessage)
	doneMessageCallbacks              []func(m *DoneMessage)
	matchMessageCallbacks             []func(m *MatchMessage)
	changeMessageCallbacks            []func(m *ChangeMessage)
	activateMessageCallbacks          []func(m *ActivateMessage)
	balanceMessageCallbacks           []func(m *BalanceMessage)
	orderbookSnapshotMessageCallbacks []func(m *OrderBookSnapshotMessage)
	orderbookUpdateMessageCallbacks   []func(m *OrderBookUpdateMessage)

	authEnabled bool

	lockSeqNumMap      sync.Mutex // lock to protect lastSequenceMsgMap
	lastSequenceMsgMap map[string]SequenceNumberType

	// TODO: replace it with exchange.activeOrderStore
	lockWorkingOrderMap sync.Mutex // lock to protect lastOrderMap
	workingOrdersMap    map[string]types.Order

	// privateChannelSymbols is a list of global symbols
	privateChannelSymbols []string

	klineCtx     context.Context
	klineCancel  context.CancelFunc
	klineDrivers []*klinedriver.TickKLineDriver

	// marketInfoMap: local symbol -> market info
	marketInfoMap map[string]*api.MarketInfo
}

func NewStream(
	exchange *Exchange,
	apiKey string,
	secretKey string,
	passphrase string,
) *Stream {
	s := Stream{
		StandardStream: types.NewStandardStream(),
		exchange:       exchange,
		apiKey:         apiKey,
		passphrase:     passphrase,
		secretKey:      secretKey,
		logger: logrus.WithFields(logrus.Fields{
			"exchange": ID,
			"module":   "stream",
		}),
		authEnabled:         len(apiKey) > 0 && len(passphrase) > 0 && len(secretKey) > 0,
		lastSequenceMsgMap:  make(map[string]SequenceNumberType),
		workingOrdersMap:    make(map[string]types.Order),
		channel2LocalIdsMap: make(map[types.Channel][]string),
		marketInfoMap:       make(map[string]*api.MarketInfo),
	}
	s.SetParser(parseMessage)
	s.SetDispatcher(s.dispatchEvent)
	s.SetEndpointCreator(s.createEndpoint)
	s.SetHeartBeat(s.ping)

	// private handlers
	s.OnErrorMessage(s.logErrorMessage)
	s.OnSubscriptions(s.logSubscriptions)
	s.OnTickerMessage(s.handleTickerMessage)
	s.OnMatchMessage(s.handleMatchMessage)
	s.OnOrderbookSnapshotMessage(s.handleOrderBookSnapshotMessage)
	s.OnOrderbookUpdateMessage(s.handleOrderbookUpdateMessage)
	s.OnBalanceMessage(s.handleBalanceMessage)
	s.OnReceivedMessage(s.handleReceivedMessage)
	s.OnOpenMessage(s.handleOpenMessage)
	s.OnDoneMessage(s.handleDoneMessage)
	s.OnChangeMessage(s.handleChangeMessage)
	s.OnActivateMessage(s.handleActivateMessage)

	// public handlers
	s.SetBeforeConnect(s.beforeConnect)
	s.OnConnect(s.handleConnect)
	s.OnDisconnect(s.handleDisconnect)
	return &s
}

func (s *Stream) Connect(ctx context.Context) error {
	if err := s.StandardStream.Connect(ctx); err != nil {
		return err
	}
	// start balance polling update worker
	// NOTE: The polling is required. The documentation clearly states that
	// the balance channel *does not* track every updates to the balance:
	// https://docs.cdp.coinbase.com/exchange/websocket-feed/channels#balance-channel
	// rate limit on private endpoints is 15 requests per second:
	// https://docs.cdp.coinbase.com/exchange/rest-api/rate-limits#private-endpoints
	go func() {
		logger := util.NewWarnFirstLogger(5, time.Minute, s.logger)
		ticker := time.NewTicker(time.Second * 5)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				balances, err := s.exchange.QueryAccountBalances(ctx)
				if err != nil {
					logger.WarnOrError(err, "failed to query account balances")
					continue
				}
				s.EmitBalanceSnapshot(balances)
			}
		}

	}()
	return nil
}

// types.PrivateChannelSymbolSetter
func (s *Stream) SetPrivateChannelSymbols(symbols []string) {
	s.privateChannelSymbols = symbols
}

func (s *Stream) privateChannelLocalSymbols() (localSymbols []string) {
	for _, symbol := range s.privateChannelSymbols {
		localSymbols = append(localSymbols, toLocalSymbol(symbol))
	}
	return
}

func (s *Stream) logSubscriptions(m *SubscriptionsMessage) {
	if m == nil {
		return
	}

	for _, channel := range m.Channels {
		s.logger.Infof("confirmed subscription: %s", channel)
	}
}

func (s *Stream) logErrorMessage(m *ErrorMessage) {
	if m == nil {
		return
	}

	s.logger.Errorf("get error message: %s", m.Reason)
}

func (s *Stream) dispatchEvent(e interface{}) {
	switch e := e.(type) {
	case *ErrorMessage:
		s.EmitErrorMessage(e)
	case *SubscriptionsMessage:
		s.EmitSubscriptions(e)
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
	case *ActivateMessage:
		s.EmitActivateMessage(e)
	case *BalanceMessage:
		s.EmitBalanceMessage(e)
	case *OrderBookSnapshotMessage:
		s.EmitOrderbookSnapshotMessage(e)
	case *OrderBookUpdateMessage:
		s.EmitOrderbookUpdateMessage(e)
	default:
		s.logger.Warnf("skip dispatching msg due to unknown message type: %T", e)
	}
}

func (s *Stream) createEndpoint(ctx context.Context) (string, error) {
	return wsFeedUrl, nil
}

func (s *Stream) generateSignature() (string, string) {
	if len(s.apiKey) == 0 || len(s.passphrase) == 0 || len(s.secretKey) == 0 {
		return "", ""
	}
	// Convert current time to string timestamp
	ts := fmt.Sprintf("%d", time.Now().Unix())

	// Create message string
	message := ts + "GET/users/self/verify"

	// Decode base64 secret
	secretBytes, err := base64.StdEncoding.DecodeString(s.secretKey)
	if err != nil {
		s.logger.WithError(err).Error("failed to decode secret key")
		return "", ""
	}

	// Create HMAC-SHA256
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(message))

	// Get signature and encode to base64
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature, ts
}

func (s *Stream) ping(conn *websocket.Conn) error {
	writeWait := 10 * time.Second

	err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait))
	if err != nil {
		s.logger.WithError(err).Error("ping error")
		return err
	}
	return nil
}
