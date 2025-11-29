package types

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// helper to start a minimal gorilla/websocket server for tests
func startTestWSServer(t *testing.T, onConn func(*websocket.Conn)) (wsURL string, closeFn func()) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		if onConn != nil {
			onConn(c)
		}
	}))

	u, _ := url.Parse(srv.URL)
	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
	return u.String(), srv.Close
}

func TestStandardStream_RawMessage_NoParser(t *testing.T) {
	msg := "hello"
	serverURL, closeServer := startTestWSServer(t, func(c *websocket.Conn) {
		defer c.Close()
		// write one text frame then keep the connection a short while
		_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
		time.Sleep(200 * time.Millisecond)
	})
	defer closeServer()

	s := NewStandardStream()
	s.SetEndpointCreator(func(ctx context.Context) (string, error) { return serverURL, nil })

	got := make(chan string, 1)
	s.OnRawMessage(func(raw []byte) { got <- string(raw) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer s.Close()

	select {
	case g := <-got:
		if g != msg {
			t.Fatalf("unexpected raw message: %q", g)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for raw message")
	}
}

func TestStandardStream_ParserAndDispatcher(t *testing.T) {
	type ev struct{ X int `json:"x"` }
	serverURL, closeServer := startTestWSServer(t, func(c *websocket.Conn) {
		defer c.Close()
		_ = c.WriteMessage(websocket.TextMessage, []byte(`{"x":42}`))
		// give client time to read
		time.Sleep(200 * time.Millisecond)
	})
	defer closeServer()

	s := NewStandardStream()
	s.SetEndpointCreator(func(ctx context.Context) (string, error) { return serverURL, nil })

	s.SetParser(func(message []byte) (interface{}, error) {
		var e ev
		if err := jsonUnmarshal(message, &e); err != nil {
			return nil, err
		}
		return &e, nil
	})

	got := make(chan int, 1)
	s.SetDispatcher(func(e interface{}) {
		if v, ok := e.(*ev); ok {
			got <- v.X
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer s.Close()

	select {
	case x := <-got:
		if x != 42 {
			t.Fatalf("unexpected parsed value: %d", x)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for dispatched event")
	}
}

// lightweight json unmarshal helper
func jsonUnmarshal(b []byte, v interface{}) error { return json.Unmarshal(b, v) }

func TestStandardStream_BeforeConnectCalled(t *testing.T) {
	serverURL, closeServer := startTestWSServer(t, func(c *websocket.Conn) {
		defer c.Close()
		time.Sleep(100 * time.Millisecond)
	})
	defer closeServer()

	s := NewStandardStream()
	s.SetEndpointCreator(func(ctx context.Context) (string, error) { return serverURL, nil })

	var called int32
	s.SetBeforeConnect(func(ctx context.Context) error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer s.Close()

	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("beforeConnect not called once, got %d", called)
	}
}

func TestStandardStream_HeartbeatAndPing(t *testing.T) {
	var pingCount int32
	serverURL, closeServer := startTestWSServer(t, func(c *websocket.Conn) {
		// count incoming pings using PingHandler
		c.SetPingHandler(func(appData string) error {
			atomic.AddInt32(&pingCount, 1)
			// reply with pong
			return c.WriteControl(websocket.PongMessage, nil, time.Now().Add(time.Second))
		})
		// keep open a bit
		defer c.Close()
		// loop read to allow control frames processing timing
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				return
			}
		}
	})
	defer closeServer()

	s := NewStandardStream()
	s.SetEndpointCreator(func(ctx context.Context) (string, error) { return serverURL, nil })
	// shorten interval for test
	s.SetPingInterval(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer s.Close()

	// wait a little for a few pings
	time.Sleep(300 * time.Millisecond)
	if atomic.LoadInt32(&pingCount) == 0 {
		t.Fatal("expected at least one ping from client")
	}
}

func TestStandardStream_ResubscribeTriggersReconnect(t *testing.T) {
	s := NewStandardStream()
	// initial subs
	s.Subscribe(Channel("book"), "BTCUSDT", SubscribeOptions{})

	done := make(chan struct{})
	go func() {
		// expect a signal on ReconnectC
		select {
		case <-s.ReconnectC:
			close(done)
		case <-time.After(2 * time.Second):
		}
	}()

	// update subs via Resubscribe
	err := s.Resubscribe(func(old []Subscription) ([]Subscription, error) {
		return append(old, Subscription{Channel: Channel("trade"), Symbol: "ETHUSDT"}), nil
	})
	if err != nil {
		t.Fatalf("resubscribe error: %v", err)
	}

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("expected reconnect signal after resubscribe")
	}
}

func TestStandardStream_Close(t *testing.T) {
	serverURL, closeServer := startTestWSServer(t, func(c *websocket.Conn) {
		defer c.Close()
		// keep alive for a bit
		time.Sleep(200 * time.Millisecond)
	})
	defer closeServer()

	s := NewStandardStream()
	s.SetEndpointCreator(func(ctx context.Context) (string, error) { return serverURL, nil })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}
}
