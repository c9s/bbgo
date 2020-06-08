package websocket

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"***REMOVED***/pkg/log"
	"***REMOVED***/pkg/testing/testutil"

	"github.com/gorilla/websocket"
)

func messageTicker(t *testing.T, ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case tt := <-ticker.C:
			err := conn.WriteMessage(websocket.TextMessage, []byte(tt.String()))
			assert.NoError(t, err)
		case <-ctx.Done():
			return
		}
	}
}

func messageReader(t *testing.T, ctx context.Context, conn *websocket.Conn) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Errorf("unexpected closed error: %v", err)
			}
			break
		}
		t.Logf("message: %v", message)
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func TestReconnect(t *testing.T) {
	basectx := context.Background()

	wsHandler := func(ctx context.Context) func(w http.ResponseWriter, r *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			var upgrader = websocket.Upgrader{
				ReadBufferSize:  1024,
				WriteBufferSize: 1024,
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("upgrade error: %v", err)
				return
			}
			go messageTicker(t, ctx, conn)
			messageReader(t, ctx, conn)
		}
	}

	serverctx, cancelserver := context.WithCancel(basectx)
	server := testutil.NewWebSocketServerFunc(t, 0, "/ws", wsHandler(serverctx))
	go func() {
		err := server.ListenAndServe()
		assert.Equal(t, http.ErrServerClosed, err)
	}()

	_, port, err := net.SplitHostPort(server.Addr)
	assert.NoError(t, err)

	log.Debugf("waiting for server port ready")
	testutil.WaitForPort(t, server.Addr, 3)

	u := url.URL{Scheme: "ws", Host: net.JoinHostPort("127.0.0.1", port), Path: "/ws"}
	log.Infof("url: %s", u.String())

	client := New(u.String(), nil)

	// start the message reader
	clientctx, cancelClient := context.WithCancel(basectx)
	defer cancelClient()

	err = client.Connect(clientctx)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	client.SetReadTimeout(2 * time.Second)

	// read one message
	log.Debugf("waiting for message...")
	msg := <-client.messages
	log.Debugf("received message: %v", msg)

	triggerReconnectManyTimes := func() {
		// Forcedly reconnect multiple times
		for i := 0; i < 10; i = i + 1 {
			client.Reconnect()
		}
	}

	log.Debugf("reconnecting...")
	triggerReconnectManyTimes()
	msg = <-client.messages
	log.Debugf("received message: %v", msg)

	log.Debugf("shutting down server...")
	err = server.Shutdown(serverctx)
	assert.NoError(t, err)
	cancelserver()
	server.Close()

	time.Sleep(500 * time.Millisecond)

	triggerReconnectManyTimes()

	log.Debugf("restarting server...")
	server2ctx, cancelserver2 := context.WithCancel(basectx)
	server2 := testutil.NewWebSocketServerWithAddressFunc(t, server.Addr, "/ws", wsHandler(server2ctx))
	go func() {
		err := server2.ListenAndServe()
		assert.Equal(t, http.ErrServerClosed, err)
	}()

	triggerReconnectManyTimes()

	log.Debugf("waiting for server2 port ready")
	testutil.WaitForPort(t, server.Addr, 3)

	log.Debugf("waiting for message from server2...")
	msg = <-client.messages
	log.Debugf("received message: %v", msg)

	err = client.Close()
	assert.NoError(t, err)

	err = server2.Shutdown(server2ctx)
	assert.NoError(t, err)
	cancelserver2()
}

func TestConnect(t *testing.T) {
	basectx := context.Background()
	ctx, cancel := context.WithCancel(basectx)

	server := testutil.NewWebSocketServerFunc(t, 0, "/ws", func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}

		go messageTicker(t, ctx, conn)
		messageReader(t, ctx, conn)
	})

	go func() {
		err := server.ListenAndServe()
		assert.Equal(t, http.ErrServerClosed, err)
	}()

	_, port, err := net.SplitHostPort(server.Addr)
	assert.NoError(t, err)

	testutil.WaitForPort(t, server.Addr, 3)

	u := url.URL{Scheme: "ws", Host: net.JoinHostPort("127.0.0.1", port), Path: "/ws"}
	t.Logf("url: %s", u.String())

	client := New(u.String(), nil)

	err = client.Connect(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	client.SetReadTimeout(2 * time.Second)

	// read one message
	t.Logf("waiting for message...")
	msg := <-client.messages
	t.Logf("received message: %v", msg)

	// recreate server at the same address
	client.Close()

	t.Logf("shutting down server...")
	cancel()
	err = server.Shutdown(basectx)
	assert.NoError(t, err)

}
