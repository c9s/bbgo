package max

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var ErrMessageTypeNotSupported = errors.New("message type currently not supported")

var logger = log.WithField("exchange", "max")

// Subscription is used for presenting the subscription metadata.
// This is used for sending subscribe and unsubscribe requests
type Subscription struct {
	Channel string `json:"channel"`
	Market  string `json:"market"`
	Depth   int    `json:"depth,omitempty"`
}

type WebsocketCommand struct {
	// Action is used for specify the action of the websocket session.
	// Valid values are "subscribe", "unsubscribe" and "auth"
	Action        string         `json:"action"`
	Subscriptions []Subscription `json:"subscriptions,omitempty"`
}

var SubscribeAction = "subscribe"
var UnsubscribeAction = "unsubscribe"

//go:generate callbackgen -type PublicWebSocketService
type PublicWebSocketService struct {
	BaseURL string

	Conn *websocket.Conn

	reconnectC chan struct{}

	// Subscriptions is the subscription request payloads that will be used for sending subscription request
	Subscriptions []Subscription

	parser PublicParser

	errorCallbacks             []func(err error)
	messageCallbacks           []func(message []byte)
	bookEventCallbacks         []func(e BookEvent)
	tradeEventCallbacks        []func(e TradeEvent)
	errorEventCallbacks        []func(e ErrorEvent)
	subscriptionEventCallbacks []func(e SubscriptionEvent)
}

func NewPublicWebSocketService(wsURL string) *PublicWebSocketService {
	return &PublicWebSocketService{
		reconnectC: make(chan struct{}, 1),
		BaseURL:    wsURL,
	}
}

func (s *PublicWebSocketService) Connect(ctx context.Context) error {
	// pre-allocate the websocket client, the websocket client can be used for reconnecting.
	go s.read(ctx)
	return s.connect(ctx)
}

func (s *PublicWebSocketService) connect(ctx context.Context) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, s.BaseURL, nil)
	if err != nil {
		return err
	}

	s.Conn = conn
	return nil
}

func (s *PublicWebSocketService) emitReconnect() {
	select {
	case s.reconnectC <- struct{}{}:
	default:
	}
}

func (s *PublicWebSocketService) read(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-s.reconnectC:
			time.Sleep(3 * time.Second)
			if err := s.connect(ctx); err != nil {
				s.emitReconnect()
			}

		default:
			mt, msg, err := s.Conn.ReadMessage()

			if err != nil {
				s.emitReconnect()
				continue
			}

			if mt != websocket.TextMessage {
				continue
			}

			s.EmitMessage(msg)

			m, err := s.parser.Parse(msg)
			if err != nil {
				s.EmitError(errors.Wrapf(err, "failed to parse public message: %s", msg))
				continue
			}

			s.dispatch(m)
		}
	}
}

func (s *PublicWebSocketService) dispatch(msg interface{}) {
	switch e := msg.(type) {
	case *BookEvent:
		s.EmitBookEvent(*e)
	case *TradeEvent:
		s.EmitTradeEvent(*e)
	case *ErrorEvent:
		s.EmitErrorEvent(*e)
	case *SubscriptionEvent:
		s.EmitSubscriptionEvent(*e)
	default:
		s.EmitError(errors.Errorf("unsupported %T event: %+v", e, e))
	}
}

func (s *PublicWebSocketService) ClearSubscriptions() {
	s.Subscriptions = nil
}

func (s *PublicWebSocketService) Reconnect() {
	logger.Info("reconnecting...")
	s.emitReconnect()
}

// Subscribe is a helper method for building subscription request from the internal mapping types.
// (Internal public method)
func (s *PublicWebSocketService) Subscribe(channel string, market string) error {
	s.AddSubscription(Subscription{
		Channel: channel,
		Market:  market,
	})
	return nil
}

// AddSubscription adds the subscription request to the buffer, these requests will be sent to the server right after connecting to the endpoint.
func (s *PublicWebSocketService) AddSubscription(subscription Subscription) {
	s.Subscriptions = append(s.Subscriptions, subscription)
}

func (s *PublicWebSocketService) Resubscribe() {
	// Calling Resubscribe() by websocket is not enough to refresh orderbook.
	// We still need to get orderbook snapshot by rest client.
	// Therefore Reconnect() is used to simplify implementation.
	logger.Info("resubscribing all subscription...")
	if err := s.SendSubscriptionRequest(UnsubscribeAction); err != nil {
		logger.WithError(err).Error("failed to unsubscribe")
	}

	if err := s.SendSubscriptionRequest(SubscribeAction); err != nil {
		logger.WithError(err).Error("failed to unsubscribe")
	}
}

func (s *PublicWebSocketService) SendSubscriptionRequest(action string) error {
	request := WebsocketCommand{
		Action:        action,
		Subscriptions: s.Subscriptions,
	}

	logger.Debugf("sending websocket subscription: %+v", request)

	if err := s.Conn.WriteJSON(request); err != nil {
		return errors.Wrap(err, "Failed to send subscribe event")
	}

	return nil
}

// Close web socket connection
func (s *PublicWebSocketService) Close() error {
	return s.Conn.Close()
}
