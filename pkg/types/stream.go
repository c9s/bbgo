package types

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const pingInterval = 30 * time.Second
const readTimeout = 2 * time.Minute
const writeTimeout = 10 * time.Second
const reconnectCoolDownPeriod = 15 * time.Second

var defaultDialer = &websocket.Dialer{
	Proxy:            http.ProxyFromEnvironment,
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   4096,
}

type Stream interface {
	StandardStreamEventHub

	Subscribe(channel Channel, symbol string, options SubscribeOptions)
	SetPublicOnly()
	Connect(ctx context.Context) error
	Close() error
}

type EndpointCreator func(ctx context.Context) (string, error)

type Parser func(message []byte) (interface{}, error)

type Dispatcher func(e interface{})

//go:generate callbackgen -type StandardStream -interface
type StandardStream struct {
	parser     Parser
	dispatcher Dispatcher

	endpointCreator EndpointCreator

	// Conn is the websocket connection
	Conn *websocket.Conn

	// ConnCtx is the context of the current websocket connection
	ConnCtx context.Context

	// ConnCancel is the cancel funcion of the current websocket connection
	ConnCancel context.CancelFunc

	// ConnLock is used for locking Conn, ConnCtx and ConnCancel fields.
	// When changing these field values, be sure to call ConnLock
	ConnLock sync.Mutex

	PublicOnly bool

	// ReconnectC is a signal channel for reconnecting
	ReconnectC chan struct{}

	// CloseC is a signal channel for closing stream
	CloseC chan struct{}

	Subscriptions []Subscription

	startCallbacks []func()

	connectCallbacks []func()

	disconnectCallbacks []func()

	// private trade update callbacks
	tradeUpdateCallbacks []func(trade Trade)

	// private order update callbacks
	orderUpdateCallbacks []func(order Order)

	// balance snapshot callbacks
	balanceSnapshotCallbacks []func(balances BalanceMap)

	balanceUpdateCallbacks []func(balances BalanceMap)

	kLineClosedCallbacks []func(kline KLine)

	kLineCallbacks []func(kline KLine)

	bookUpdateCallbacks []func(book SliceOrderBook)

	bookTickerUpdateCallbacks []func(bookTicker BookTicker)

	bookSnapshotCallbacks []func(book SliceOrderBook)

	marketTradeCallbacks []func(trade Trade)

	// Futures
	FuturesPositionUpdateCallbacks []func(futuresPositions FuturesPositionMap)

	FuturesPositionSnapshotCallbacks []func(futuresPositions FuturesPositionMap)
}

func NewStandardStream() StandardStream {
	return StandardStream{
		ReconnectC: make(chan struct{}, 1),
		CloseC:     make(chan struct{}),
	}
}

func (s *StandardStream) SetPublicOnly() {
	s.PublicOnly = true
}

func (s *StandardStream) SetEndpointCreator(creator EndpointCreator) {
	s.endpointCreator = creator
}

func (s *StandardStream) SetDispatcher(dispatcher Dispatcher) {
	s.dispatcher = dispatcher
}

func (s *StandardStream) SetParser(parser Parser) {
	s.parser = parser
}

func (s *StandardStream) SetConn(ctx context.Context, conn *websocket.Conn) (context.Context, context.CancelFunc) {
	// should only start one connection one time, so we lock the mutex
	connCtx, connCancel := context.WithCancel(ctx)
	s.ConnLock.Lock()

	// ensure the previous context is cancelled
	if s.ConnCancel != nil {
		s.ConnCancel()
	}

	// create a new context for this connection
	s.Conn = conn
	s.ConnCtx = connCtx
	s.ConnCancel = connCancel
	s.ConnLock.Unlock()
	return connCtx, connCancel
}

func (s *StandardStream) Read(ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc) {
	defer func() {
		cancel()
		s.EmitDisconnect()
	}()

	// flag format: debug-{component}-{message type}
	debugRawMessage := viper.GetBool("debug-websocket-raw-message")

	for {
		select {

		case <-ctx.Done():
			return

		case <-s.CloseC:
			return

		default:
			if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
				log.WithError(err).Errorf("set read deadline error: %s", err.Error())
			}

			mt, message, err := conn.ReadMessage()
			if err != nil {
				// if it's a network timeout error, we should re-connect
				switch err := err.(type) {

				// if it's a websocket related error
				case *websocket.CloseError:
					if err.Code == websocket.CloseNormalClosure {
						return
					}

					log.WithError(err).Errorf("websocket error abnormal close: %+v", err)

					_ = conn.Close()
					// for unexpected close error, we should re-connect
					// emit reconnect to start a new connection
					s.Reconnect()
					return

				case net.Error:
					log.WithError(err).Error("websocket read network error")
					_ = conn.Close()
					s.Reconnect()
					return

				default:
					log.WithError(err).Error("unexpected websocket error")
					_ = conn.Close()
					s.Reconnect()
					return
				}
			}

			// skip non-text messages
			if mt != websocket.TextMessage {
				continue
			}

			if debugRawMessage {
				log.Info(string(message))
			}

			var e interface{}
			if s.parser != nil {
				e, err = s.parser(message)
				if err != nil {
					log.WithError(err).Errorf("websocket event parse error")
					continue
				}
			}

			if s.dispatcher != nil {
				s.dispatcher(e)
			}
		}
	}
}

func (s *StandardStream) ping(ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc, interval time.Duration) {
	defer func() {
		cancel()
		log.Debug("ping worker stopped")
	}()

	var pingTicker = time.NewTicker(interval)
	defer pingTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-s.CloseC:
			return

		case <-pingTicker.C:
			log.Debugf("websocket -> ping")
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				log.WithError(err).Error("ping error", err)
				s.Reconnect()
			}
		}
	}
}

func (s *StandardStream) Subscribe(channel Channel, symbol string, options SubscribeOptions) {
	s.Subscriptions = append(s.Subscriptions, Subscription{
		Channel: channel,
		Symbol:  symbol,
		Options: options,
	})
}

func (s *StandardStream) Reconnect() {
	select {
	case s.ReconnectC <- struct{}{}:
	default:
	}
}

// Connect starts the stream and create the websocket connection
func (s *StandardStream) Connect(ctx context.Context) error {
	err := s.DialAndConnect(ctx)
	if err != nil {
		return err
	}

	// start one re-connector goroutine with the base context
	go s.reconnector(ctx)

	s.EmitStart()
	return nil
}

func (s *StandardStream) reconnector(ctx context.Context) {
	for {
		select {

		case <-ctx.Done():
			return

		case <-s.CloseC:
			return

		case <-s.ReconnectC:
			log.Warnf("received reconnect signal, cooling for %s...", reconnectCoolDownPeriod)
			time.Sleep(reconnectCoolDownPeriod)

			log.Warnf("re-connecting...")
			if err := s.DialAndConnect(ctx); err != nil {
				log.WithError(err).Errorf("re-connect error, try to reconnect later")

				// re-emit the re-connect signal if error
				s.Reconnect()
			}
		}
	}
}

func (s *StandardStream) DialAndConnect(ctx context.Context) error {
	conn, err := s.Dial(ctx)
	if err != nil {
		return err
	}

	connCtx, connCancel := s.SetConn(ctx, conn)
	s.EmitConnect()

	go s.Read(connCtx, conn, connCancel)
	go s.ping(connCtx, conn, connCancel, pingInterval)
	return nil
}

func (s *StandardStream) Dial(ctx context.Context, args ...string) (*websocket.Conn, error) {
	var url string
	var err error
	if len(args) > 0 {
		url = args[0]
	} else if s.endpointCreator != nil {
		url, err = s.endpointCreator(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "can not dial, can not create endpoint via the endpoint creator")
		}
	} else {
		return nil, errors.New("can not dial, neither url nor endpoint creator is not defined, you should pass an url to Dial() or call SetEndpointCreator()")
	}

	conn, _, err := defaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	// use the default ping handler
	// The websocket server will send a ping frame every 3 minutes.
	// If the websocket server does not receive a pong frame back from the connection within a 10 minutes period,
	// the connection will be disconnected.
	// Unsolicited pong frames are allowed.
	conn.SetPingHandler(nil)
	conn.SetPongHandler(func(string) error {
		log.Debugf("websocket <- received pong")
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout * 2)); err != nil {
			log.WithError(err).Error("pong handler can not set read deadline")
		}
		return nil
	})

	log.Infof("websocket connected, public = %v, read timeout = %v", s.PublicOnly, readTimeout)
	return conn, nil
}

func (s *StandardStream) Close() error {
	log.Debugf("closing stream...")

	// close the close signal channel, so that reader and ping worker will stop
	close(s.CloseC)

	// get the connection object before call the context cancel function
	s.ConnLock.Lock()
	conn := s.Conn
	connCancel := s.ConnCancel
	s.ConnLock.Unlock()

	// cancel the context so that the ticker loop and listen key updater will be stopped.
	if connCancel != nil {
		connCancel()
	}

	// gracefully write the close message to the connection
	err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return errors.Wrap(err, "websocket write close message error")
	}

	log.Debugf("stream closed")

	// let the reader close the connection
	<-time.After(time.Second)
	return nil
}

type Depth string

const (
	DepthLevelFull   Depth = "FULL"
	DepthLevelMedium Depth = "MEDIUM"
	DepthLevel1      Depth = "1"
	DepthLevel5      Depth = "5"
	DepthLevel20     Depth = "20"
)

type Speed string

const (
	SpeedHigh   Speed = "HIGH"
	SpeedMedium Speed = "MEDIUM"
	SpeedLow    Speed = "LOW"
)

// SubscribeOptions provides the standard stream options
type SubscribeOptions struct {
	Interval string `json:"interval,omitempty"`
	Depth    Depth  `json:"depth,omitempty"`
	Speed    Speed  `json:"speed,omitempty"`
}

func (o SubscribeOptions) String() string {
	if len(o.Interval) > 0 {
		return o.Interval
	}

	return string(o.Depth)
}

type Subscription struct {
	Symbol  string           `json:"symbol"`
	Channel Channel          `json:"channel"`
	Options SubscribeOptions `json:"options"`
}
