package websocket

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/log"
	"github.com/c9s/bbgo/pkg/util"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const DefaultMessageBufferSize = 128

const DefaultWriteTimeout = 30 * time.Second
const DefaultReadTimeout = 30 * time.Second

var ErrReconnectContextDone = errors.New("reconnect canceled due to context done.")
var ErrReconnectFailed = errors.New("failed to reconnect")
var ErrConnectionLost = errors.New("connection lost")

var MaxReconnectRate = rate.Limit(1 / DefaultMinBackoff.Seconds())

// WebSocketClient allows to connect and receive stream data
type WebSocketClient struct {
	// Url is the websocket connection location, start with ws:// or wss://
	Url string

	// conn is the current websocket connection, please note the connection
	// object can be replaced with a new connection object when the connection
	// is unexpected closed.
	conn *websocket.Conn

	// Dialer is used for creating the websocket connection
	Dialer *websocket.Dialer

	// requestHeader is used for the Dial function call. Some credential can be
	// stored in the http request header for authentication
	requestHeader http.Header

	// messages is a read-only channel, received messages will be sent to this
	// channel.
	messages chan Message

	readTimeout time.Duration

	writeTimeout time.Duration

	onConnect []func(c Client)

	onDisconnect []func(c Client)

	// cancel is mapped to the ctx context object
	cancel func()

	readerClosed chan struct{}

	connected bool

	mu sync.Mutex

	reconnectCh chan struct{}

	backoff Backoff

	limiter *rate.Limiter
}

type Message struct {
	// websocket.BinaryMessage or websocket.TextMessage
	Type int
	Body []byte
}

func (c *WebSocketClient) Messages() <-chan Message {
	return c.messages
}

func (c *WebSocketClient) SetReadTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readTimeout = timeout
}

func (c *WebSocketClient) SetWriteTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeTimeout = timeout
}

func (c *WebSocketClient) OnConnect(f func(c Client)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onConnect = append(c.onConnect, f)
}

func (c *WebSocketClient) OnDisconnect(f func(c Client)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onDisconnect = append(c.onDisconnect, f)
}

func (c *WebSocketClient) WriteTextMessage(message []byte) error {
	return c.WriteMessage(websocket.TextMessage, message)
}

func (c *WebSocketClient) WriteBinaryMessage(message []byte) error {
	return c.WriteMessage(websocket.BinaryMessage, message)
}

func (c *WebSocketClient) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return ErrConnectionLost
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout)); err != nil {
		return err
	}
	return c.conn.WriteMessage(messageType, data)
}

func (c *WebSocketClient) readMessages() error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return ErrConnectionLost
	}
	timeout := c.readTimeout
	conn := c.conn
	c.mu.Unlock()

	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	msgtype, message, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	c.messages <- Message{msgtype, message}
	return nil
}

// listen starts a goroutine for reading message and tries to re-connect to the
// server when the reader returns error
//
// Please note we should always break the reader loop if there is any error
// returned from the server.
func (c *WebSocketClient) listen(ctx context.Context) {
	// The life time of both channels "readerClosed" and "reconnectCh" is bound to one connection.
	// Each channel should be created before loop starts and be closed after loop ends.
	// "readerClosed" is used to inform "Close()" reader loop ends.
	// "reconnectCh" is used to centralize reconnection logics in this reader loop.
	c.mu.Lock()
	c.readerClosed = make(chan struct{})
	c.reconnectCh = make(chan struct{}, 1)
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		close(c.readerClosed)
		close(c.reconnectCh)
		c.reconnectCh = nil
		c.mu.Unlock()
	}()

	for {
		select {

		case <-ctx.Done():
			return

		case <-c.reconnectCh:
			// it could be i/o timeout for network disconnection
			// or it could be invoked from outside.
			c.SetDisconnected()
			var maxTries = 1
			if _, response, err := c.reconnect(ctx, maxTries); err != nil {
				if err == ErrReconnectContextDone {
					log.Debugf("[websocket] context canceled. stop reconnecting.")
					return
				}
				log.Warnf("[websocket] failed to reconnect after %d tries!! error: %v response: %v", maxTries, err, response)
				c.Reconnect()
			}

		default:
			if err := c.readMessages(); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
					log.Warnf("[websocket] unexpected close error reconnecting: %v", err)
				}

				log.Warnf("[websocket] failed to read message. error: %+v", err)
				c.Reconnect()
			}
		}
	}
}

// Reconnect triggers reconnection logics
func (c *WebSocketClient) Reconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	// c.reconnectCh is a buffered channel with cap=1.
	// At most one reconnect signal could be processed.
	case c.reconnectCh <- struct{}{}:
	default:
		// Entering here means it is already reconnecting.
		// Drop the current reconnect signal.
	}
}

// Close gracefully shuts down the reader and the connection
// ctx is the context used for shutdown process.
func (c *WebSocketClient) Close() (err error) {
	c.mu.Lock()
	// leave the listen goroutine before we close the connection
	// checking nil is to handle calling "Close" before "Connect" is called
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()
	c.SetDisconnected()

	// wait for the reader func to be closed
	if c.readerClosed != nil {
		<-c.readerClosed
	}
	return err
}

// reconnect tries to create a new connection from the existing dialer
func (c *WebSocketClient) reconnect(ctx context.Context, maxTries int) (*websocket.Conn, *http.Response, error) {
	log.Debugf("[websocket] start reconnecting to %q", c.Url)

	select {
	case <-ctx.Done():
		return nil, nil, ErrReconnectContextDone
	default:
	}

	if s := util.ShouldDelay(c.limiter, DefaultMinBackoff); s > 0 {
		log.Warn("[websocket] reconnect too frequently. Sleep for ", s)
		time.Sleep(s)
	}

	log.Warnf("[websocket] reconnecting x %d to %q", c.backoff.Attempt()+1, c.Url)
	conn, resp, err := c.Dialer.DialContext(ctx, c.Url, c.requestHeader)
	if err != nil {
		dur := c.backoff.Duration()
		log.Warnf("failed to dial %s: %v, response: %+v. Wait for %v", c.Url, err, resp, dur)
		time.Sleep(dur)
		return nil, nil, ErrReconnectFailed
	}

	log.Infof("[websocket] reconnected to %q", c.Url)
	// Reset backoff value if connected.
	c.backoff.Reset()
	c.setConn(conn)
	c.setPingHandler(conn)

	return conn, resp, err
}

// Conn returns the current active connection instance
func (c *WebSocketClient) Conn() (conn *websocket.Conn) {
	c.mu.Lock()
	conn = c.conn
	c.mu.Unlock()
	return conn
}

func (c *WebSocketClient) setConn(conn *websocket.Conn) {
	// Disconnect old connection before replacing with new one.
	c.SetDisconnected()

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()
	for _, f := range c.onConnect {
		go f(c)
	}
}

func (c *WebSocketClient) setPingHandler(conn *websocket.Conn) {
	conn.SetPingHandler(func(message string) error {
		if err := conn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(time.Second)); err != nil {
			return err
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		return conn.SetReadDeadline(time.Now().Add(c.readTimeout))
	})
}

func (c *WebSocketClient) SetDisconnected() {
	c.mu.Lock()
	closed := false
	if c.conn != nil {
		closed = true
		c.conn.Close()
	}
	c.connected = false
	c.conn = nil
	c.mu.Unlock()

	if closed {
		// Only call disconnect callbacks when a connection is closed
		for _, f := range c.onDisconnect {
			go f(c)
		}
	}
}

func (c *WebSocketClient) IsConnected() (ret bool) {
	c.mu.Lock()
	ret = c.connected
	c.mu.Unlock()
	return ret
}

func (c *WebSocketClient) Connect(basectx context.Context) error {
	// maintain a context by the client it self, so that we can manually shutdown the connection
	ctx, cancel := context.WithCancel(basectx)
	c.cancel = cancel

	conn, _, err := c.Dialer.DialContext(ctx, c.Url, c.requestHeader)
	if err == nil {
		// setup connection only when connected
		c.setConn(conn)
		c.setPingHandler(conn)
	}

	// 1) if connection is built up, start listening for messages.
	// 2) if connection is NOT ready, start reconnecting infinitely.
	go c.listen(ctx)

	return err
}

func New(url string, requestHeader http.Header) *WebSocketClient {
	return NewWithDialer(url, websocket.DefaultDialer, requestHeader)
}

func NewWithDialer(url string, d *websocket.Dialer, requestHeader http.Header) *WebSocketClient {
	limiter, err := util.NewValidLimiter(MaxReconnectRate, 1)
	if err != nil {
		log.WithError(err).Panic("Invalid rate limiter")
	}
	return &WebSocketClient{
		Url:           url,
		Dialer:        d,
		requestHeader: requestHeader,
		readTimeout:   DefaultReadTimeout,
		writeTimeout:  DefaultWriteTimeout,
		messages:      make(chan Message, DefaultMessageBufferSize),
		limiter:       limiter,
	}
}
