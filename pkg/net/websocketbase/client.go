package websocketbase

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebsocketClientBase is a legacy base client
// Deprecated: please use standard stream instead.
//go:generate callbackgen -type WebsocketClientBase
type WebsocketClientBase struct {
	baseURL string

	// mu protects conn
	mu                sync.Mutex
	conn              *websocket.Conn
	reconnectC        chan struct{}
	reconnectDuration time.Duration

	connectedCallbacks    []func(conn *websocket.Conn)
	disconnectedCallbacks []func(conn *websocket.Conn)
	messageCallbacks      []func(message []byte)
	errorCallbacks        []func(err error)
}

func NewWebsocketClientBase(baseURL string, reconnectDuration time.Duration) *WebsocketClientBase {
	return &WebsocketClientBase{
		baseURL:           baseURL,
		reconnectC:        make(chan struct{}, 1),
		reconnectDuration: reconnectDuration,
	}
}

func (s *WebsocketClientBase) Listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.reconnectC:
			time.Sleep(s.reconnectDuration)
			if err := s.connect(ctx); err != nil {
				s.Reconnect()
			}
		default:
			conn := s.Conn()
			mt, msg, err := conn.ReadMessage()

			if err != nil {
				s.Reconnect()
				continue
			}

			if mt != websocket.TextMessage {
				continue
			}

			s.EmitMessage(msg)
		}
	}
}

func (s *WebsocketClientBase) Connect(ctx context.Context) error {
	if err := s.connect(ctx); err != nil {
		return err
	}
	go s.Listen(ctx)
	return nil
}

func (s *WebsocketClientBase) Reconnect() {
	select {
	case s.reconnectC <- struct{}{}:
	default:
	}
}

func (s *WebsocketClientBase) connect(ctx context.Context) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, s.baseURL, nil)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	s.EmitConnected(conn)

	return nil
}

func (s *WebsocketClientBase) Conn() *websocket.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn
}
