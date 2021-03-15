package max

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var WebSocketURL = "wss://max-stream.maicoin.com/ws"

var ErrMessageTypeNotSupported = errors.New("message type currently not supported")

type SubscribeOptions struct {
	Depth      int    `json:"depth,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

// Subscription is used for presenting the subscription metadata.
// This is used for sending subscribe and unsubscribe requests
type Subscription struct {
	Channel    string `json:"channel"`
	Market     string `json:"market"`
	Depth      int    `json:"depth,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

type WebsocketCommand struct {
	// Action is used for specify the action of the websocket session.
	// Valid values are "subscribe", "unsubscribe" and "auth"
	Action        string         `json:"action"`
	Subscriptions []Subscription `json:"subscriptions,omitempty"`
}

var SubscribeAction = "subscribe"
var UnsubscribeAction = "unsubscribe"

//go:generate callbackgen -type WebSocketService
type WebSocketService struct {
	baseURL, key, secret string

	mu   sync.Mutex
	conn *websocket.Conn

	reconnectC chan struct{}

	// Subscriptions is the subscription request payloads that will be used for sending subscription request
	Subscriptions []Subscription

	connectCallbacks    []func(conn *websocket.Conn)
	disconnectCallbacks []func()

	errorCallbacks             []func(err error)
	messageCallbacks           []func(message []byte)
	bookEventCallbacks         []func(e BookEvent)
	tradeEventCallbacks        []func(e PublicTradeEvent)
	kLineEventCallbacks        []func(e KLineEvent)
	errorEventCallbacks        []func(e ErrorEvent)
	subscriptionEventCallbacks []func(e SubscriptionEvent)

	tradeUpdateEventCallbacks   []func(e TradeUpdateEvent)
	tradeSnapshotEventCallbacks []func(e TradeSnapshotEvent)
	orderUpdateEventCallbacks   []func(e OrderUpdateEvent)
	orderSnapshotEventCallbacks []func(e OrderSnapshotEvent)

	accountSnapshotEventCallbacks []func(e AccountSnapshotEvent)
	accountUpdateEventCallbacks   []func(e AccountUpdateEvent)
}

func NewWebSocketService(wsURL string, key, secret string) *WebSocketService {
	return &WebSocketService{
		key:        key,
		secret:     secret,
		reconnectC: make(chan struct{}, 1),
		baseURL:    wsURL,
	}
}

func (s *WebSocketService) Connect(ctx context.Context) error {
	s.OnConnect(func(c *websocket.Conn) {
		if err := s.SendSubscriptionRequest(SubscribeAction); err != nil {
			s.EmitError(err)
			logger.WithError(err).Error("failed to subscribe")
		}
	})

	// pre-allocate the websocket client, the websocket client can be used for reconnecting.
	if err := s.connect(ctx); err != nil {
		return err
	}

	go s.reconnector(ctx)
	return nil
}

func (s *WebSocketService) Auth() error {
	nonce := time.Now().UnixNano() / int64(time.Millisecond)
	auth := &AuthMessage{
		Action:    "auth",
		APIKey:    s.key,
		Nonce:     nonce,
		Signature: signPayload(fmt.Sprintf("%d", nonce), s.secret),
		ID:        uuid.New().String(),
	}
	return s.conn.WriteJSON(auth)
}

func (s *WebSocketService) connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, s.baseURL, nil)
	if err != nil {
		return err
	}

	s.conn = conn
	s.EmitConnect(conn)

	go s.read(ctx)

	return nil
}

func (s *WebSocketService) emitReconnect() {
	select {
	case s.reconnectC <- struct{}{}:
	default:
	}
}

func (s *WebSocketService) reconnector(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-s.reconnectC:
			time.Sleep(3 * time.Second)
			if err := s.connect(ctx); err != nil {
				s.emitReconnect()
			}
		}
	}
}

func (s *WebSocketService) read(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			s.mu.Lock()
			mt, msg, err := s.conn.ReadMessage()
			s.mu.Unlock()

			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
					s.EmitDisconnect()
					// emit reconnect to start a new connection
					s.emitReconnect()
					return
				}

				log.WithError(err).Error("websocket error")
				continue
			}

			if mt != websocket.TextMessage {
				continue
			}

			s.EmitMessage(msg)

			m, err := ParseMessage(msg)
			if err != nil {
				s.EmitError(errors.Wrapf(err, "failed to parse message: %s", msg))
				continue
			}

			if m != nil {
				s.dispatch(m)
			}
		}
	}
}

func (s *WebSocketService) dispatch(msg interface{}) {
	switch e := msg.(type) {

	case *BookEvent:
		s.EmitBookEvent(*e)

	case *PublicTradeEvent:
		s.EmitTradeEvent(*e)

	case *KLineEvent:
		s.EmitKLineEvent(*e)

	case *ErrorEvent:
		s.EmitErrorEvent(*e)

	case *SubscriptionEvent:
		s.EmitSubscriptionEvent(*e)

	case *TradeSnapshotEvent:
		s.EmitTradeSnapshotEvent(*e)

	case *TradeUpdateEvent:
		s.EmitTradeUpdateEvent(*e)

	case *AccountSnapshotEvent:
		s.EmitAccountSnapshotEvent(*e)

	case *AccountUpdateEvent:
		s.EmitAccountUpdateEvent(*e)

	case *OrderSnapshotEvent:
		s.EmitOrderSnapshotEvent(*e)

	case *OrderUpdateEvent:
		s.EmitOrderUpdateEvent(*e)

	default:
		s.EmitError(fmt.Errorf("unsupported %T event: %+v", e, e))
	}
}

func (s *WebSocketService) ClearSubscriptions() {
	s.Subscriptions = nil
}

func (s *WebSocketService) Reconnect() {
	logger.Info("reconnecting...")
	s.emitReconnect()
}

// Subscribe is a helper method for building subscription request from the internal mapping types.
// (Internal public method)
func (s *WebSocketService) Subscribe(channel, market string, options SubscribeOptions) {
	s.AddSubscription(Subscription{
		Channel:    channel,
		Market:     market,
		Depth:      options.Depth,
		Resolution: options.Resolution,
	})
}

// AddSubscription adds the subscription request to the buffer, these requests will be sent to the server right after connecting to the endpoint.
func (s *WebSocketService) AddSubscription(subscription Subscription) {
	s.Subscriptions = append(s.Subscriptions, subscription)
}

func (s *WebSocketService) Resubscribe() {
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

func (s *WebSocketService) SendSubscriptionRequest(action string) error {
	request := WebsocketCommand{
		Action:        action,
		Subscriptions: s.Subscriptions,
	}

	logger.Debugf("sending websocket subscription: %+v", request)

	if err := s.conn.WriteJSON(request); err != nil {
		return errors.Wrap(err, "Failed to send subscribe event")
	}

	return nil
}

// Close web socket connection
func (s *WebSocketService) Close() error {
	return s.conn.Close()
}
