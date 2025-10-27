package types

import (
	"context"
	"fmt"
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

//go:generate mockgen -destination=mocks/mock_stream.go -package=mocks . Stream
type Stream interface {
	StandardStreamEventHub

	// Subscribe subscribes the specific channel, but not connect to the server.
	Subscribe(channel Channel, symbol string, options SubscribeOptions)
	GetSubscriptions() []Subscription
	// Resubscribe used to update or renew existing subscriptions. It will reconnect to the server.
	Resubscribe(func(oldSubs []Subscription) (newSubs []Subscription, err error)) error
	// SetPublicOnly connects to public or private
	SetPublicOnly()
	GetPublicOnly() bool

	// Connect connects to websocket server
	Connect(ctx context.Context) error
	Reconnect()
	Close() error
}

type PrivateChannelSetter interface {
	SetPrivateChannels(channels []string)
}

type PrivateChannelSymbolSetter interface {
	SetPrivateChannelSymbols(symbols []string)
}

type Unsubscriber interface {
	// Unsubscribe unsubscribes the all subscriptions.
	Unsubscribe()
}

type EndpointCreator func(ctx context.Context) (string, error)

type Parser func(message []byte) (interface{}, error)

type Dispatcher func(e interface{})

// HeartBeat keeps connection alive by sending the ping packet.
type HeartBeat func(conn *websocket.Conn) error

type BeforeConnect func(ctx context.Context) error

type WebsocketPongEvent struct{}

//go:generate callbackgen -type StandardStream -interface
type StandardStream struct {
	parser       Parser
	dispatcher   Dispatcher
	pingInterval time.Duration

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

	// sg is used to wait until the previous routines are closed.
	// only handle routines used internally, avoid including external callback func to prevent issues if they have
	// bugs and cannot terminate.
	sg SyncGroup

	// ReconnectC is a signal channel for reconnecting
	ReconnectC chan struct{}

	// CloseC is a signal channel for closing stream
	CloseC chan struct{}

	Subscriptions []Subscription

	// subLock is used for locking Subscriptions fields.
	// When changing these field values, be sure to call subLock
	subLock sync.Mutex

	// startCallbacks are called when the stream is started
	// only called once
	startCallbacks []func()

	connectCallbacks []func()

	disconnectCallbacks []func()

	authCallbacks []func()

	rawMessageCallbacks []func(raw []byte)

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

	aggTradeCallbacks []func(trade Trade)

	forceOrderCallbacks []func(info LiquidationInfo)

	// Futures
	FuturesPositionUpdateCallbacks []func(futuresPositions FuturesPositionMap)

	FuturesPositionSnapshotCallbacks []func(futuresPositions FuturesPositionMap)

	heartBeat HeartBeat

	beforeConnect BeforeConnect
}

type StandardStreamEmitter interface {
	Stream
	EmitStart()
	EmitConnect()
	EmitDisconnect()
	EmitAuth()
	EmitTradeUpdate(Trade)
	EmitOrderUpdate(Order)
	EmitBalanceSnapshot(BalanceMap)
	EmitBalanceUpdate(BalanceMap)
	EmitKLineClosed(KLine)
	EmitKLine(KLine)
	EmitBookUpdate(SliceOrderBook)
	EmitBookTickerUpdate(BookTicker)
	EmitBookSnapshot(SliceOrderBook)
	EmitMarketTrade(Trade)
	EmitAggTrade(Trade)
	EmitForceOrder(LiquidationInfo)
	EmitFuturesPositionUpdate(FuturesPositionMap)
	EmitFuturesPositionSnapshot(FuturesPositionMap)
}

// NewStandardStream constructs a StandardStream with sane defaults.
//
// Key defaults:
// - ReconnectC: a buffered channel used as a signal to reconnect
// - CloseC: an unbuffered channel used to signal shutdown to internal goroutines
// - pingInterval: defaults to 30s
//
// After construction, typical setup is:
//
//	s.SetEndpointCreator(...)
//	s.SetParser(...)
//	s.SetDispatcher(...)
//	s.Connect(ctx)
func NewStandardStream() StandardStream {
	return StandardStream{
		ReconnectC:   make(chan struct{}, 1),
		CloseC:       make(chan struct{}),
		sg:           NewSyncGroup(),
		pingInterval: pingInterval,
	}
}

// SetPublicOnly switches the stream to public-only mode.
// In public-only mode, the stream will connect to public market data endpoints
// and skip any private/user-data setup that may require authentication.
// This should typically be called before Connect.
func (s *StandardStream) SetPublicOnly() {
	s.PublicOnly = true
}

// GetPublicOnly reports whether the stream is in public-only mode.
func (s *StandardStream) GetPublicOnly() bool {
	return s.PublicOnly
}

// SetEndpointCreator sets a function that returns the websocket endpoint URL used by Dial.
// Use this when the endpoint depends on runtime state (e.g. auth tokens, listen-keys).
// If no URL argument is passed to Dial, this creator will be invoked. If neither is provided,
// Dial returns an error.
func (s *StandardStream) SetEndpointCreator(creator EndpointCreator) {
	s.endpointCreator = creator
}

// SetDispatcher sets the dispatcher function that receives parsed events from Read.
// If set, every successfully parsed message will be passed to this dispatcher.
// The dispatcher should be non-blocking or handle its own goroutine if it may block.
func (s *StandardStream) SetDispatcher(dispatcher Dispatcher) {
	s.dispatcher = dispatcher
}

// SetParser sets the parser that converts raw text WebSocket frames into typed events.
// If not set, all text frames are emitted via OnRawMessage and not dispatched.
func (s *StandardStream) SetParser(parser Parser) {
	s.parser = parser
}

// Read runs the main read loop for a connected websocket.
// It:
// - sets read deadlines and reads text frames
// - emits OnRawMessage when no parser is set, or when parsing fails, or when the event is not a pong
// - invokes the parser (if any) and then the dispatcher (if any)
// - on read/control errors, it closes the connection and schedules a reconnect via Reconnect()
// The loop exits when ctx is done or Close() is called.
func (s *StandardStream) Read(ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc) {
	defer func() {
		cancel()
		s.EmitDisconnect()
	}()

	// flag format: debug-{component}-{message type}
	debugRawMessage := viper.GetBool("debug-websocket-raw-message")

	hasParser := s.parser != nil
	hasDispatcher := s.dispatcher != nil

	for {
		select {

		case <-ctx.Done():
			return

		case <-s.CloseC:
			return

		default:
			if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
				log.WithError(err).Errorf("unable to set read deadline: %v", err)
			}

			mt, message, err := conn.ReadMessage()
			if err != nil {
				// if it's a network timeout error, we should re-connect
				switch err2 := err.(type) {

				// if it's a websocket related error
				case *websocket.CloseError:
					if err2.Code != websocket.CloseNormalClosure {
						log.WithError(err2).Warnf("websocket error abnormal close: %+v", err2)
					}

					_ = conn.Close()
					// for close error, we should re-connect
					// emit reconnect to start a new connection
					s.Reconnect()
					return

				case net.Error:
					log.WithError(err2).Warn("websocket read network error")
					_ = conn.Close()
					s.Reconnect()
					return

				default:
					log.WithError(err2).Warn("unexpected websocket error")
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

			if !hasParser {
				s.EmitRawMessage(message)
				continue
			}

			var e interface{}
			e, err = s.parser(message)
			if err != nil {
				log.WithError(err).Errorf("unable to parse the websocket message. err: %v, message: %s", err, message)
				// emit raw message even if occurs error, because we want anything can be detected
				s.EmitRawMessage(message)
				continue
			}

			// skip pong event to avoid the message like spam
			if _, ok := e.(*WebsocketPongEvent); !ok {
				s.EmitRawMessage(message)
			}

			if hasDispatcher {
				s.dispatcher(e)
			}
		}
	}
}

// SetPingInterval overrides the default ping interval used by the ping worker.
// Useful to shorten intervals in tests or adapt to exchange-specific requirements.
func (s *StandardStream) SetPingInterval(interval time.Duration) {
	s.pingInterval = interval
}

// ping runs the periodic ping worker. On each tick it optionally calls the custom
// heartBeat (if set) and then writes a websocket ping control frame. Any error from
// the heartbeat or ping write triggers Reconnect() and exits the worker.
func (s *StandardStream) ping(
	ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc,
) {
	defer func() {
		cancel()
		log.Debug("[websocket] ping worker stopped")
	}()

	var pingTicker = time.NewTicker(s.pingInterval)
	defer pingTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-s.CloseC:
			return

		case <-pingTicker.C:
			if s.heartBeat != nil {
				if err := s.heartBeat(conn); err != nil {
					// log errors at the concrete class so that we can identify which exchange encountered an error
					s.Reconnect()
					return
				}
			}

			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				log.WithError(err).Warnf("unable to write ws control message, ping error: %v", err)
				s.Reconnect()
				return
			}
		}
	}
}

// GetSubscriptions returns the current subscription list in a thread-safe way.
// The returned slice is the internal one; treat it as read-only.
func (s *StandardStream) GetSubscriptions() []Subscription {
	s.subLock.Lock()
	defer s.subLock.Unlock()

	return s.Subscriptions
}

// Resubscribe synchronizes the new subscriptions based on the provided function.
// The fn function takes the old subscriptions as input and returns the new subscriptions that will replace the old ones
// in the struct then Reconnect.
// This method is thread-safe.
func (s *StandardStream) Resubscribe(fn func(old []Subscription) (new []Subscription, err error)) error {
	s.subLock.Lock()
	defer s.subLock.Unlock()

	var err error
	subs, err := fn(s.Subscriptions)
	if err != nil {
		return err
	}
	s.Subscriptions = subs
	s.Reconnect()
	return nil
}

// Subscribe appends a new subscription in a thread-safe manner.
// This only records the intent locally; concrete exchange streams usually
// send the broker-specific subscribe command in their OnConnect handler.
func (s *StandardStream) Subscribe(channel Channel, symbol string, options SubscribeOptions) {
	s.subLock.Lock()
	defer s.subLock.Unlock()

	s.Subscriptions = append(s.Subscriptions, Subscription{
		Channel: channel,
		Symbol:  symbol,
		Options: options,
	})
}

// Reconnect schedules a reconnection by sending a signal into ReconnectC.
// The reconnector goroutine (started by Connect) will cool down briefly and
// then attempt DialAndConnect again. Multiple calls coalesce thanks to the
// buffered channel and default case.
func (s *StandardStream) Reconnect() {
	select {
	case s.ReconnectC <- struct{}{}:
	default:
	}
}

// Connect initializes and starts the stream lifecycle.
// Steps:
// 1) Run the optional beforeConnect hook.
// 2) Dial and establish the connection, starting the read and ping workers.
// 3) Start a background reconnector goroutine listening on ReconnectC.
// 4) Emit OnStart once.
//
// Connect is non-blocking after initial dialing; it spawns goroutines.
func (s *StandardStream) Connect(ctx context.Context) error {
	if s.beforeConnect != nil {
		if err := s.beforeConnect(ctx); err != nil {
			return err
		}
	}
	err := s.DialAndConnect(ctx)
	if err != nil {
		return err
	}

	// start one re-connector goroutine with the base context
	// reconnector goroutine does not exit when the connection is closed
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
				log.WithError(err).Warnf("re-connect error: %v, will reconnect again later...", err)

				// re-emit the re-connect signal if error
				s.Reconnect()
			}
		}
	}
}

// DialAndConnect dials (using Dial) and starts the read and ping workers for the new connection.
// It emits OnConnect after installing the connection and before starting workers.
// Existing connection state, if any, is cleaned up by SetConn.
func (s *StandardStream) DialAndConnect(ctx context.Context) error {
	// should only start one connection one time, so we lock the mutex
	s.ConnLock.Lock()
	defer s.ConnLock.Unlock()

	// ensure the previous context is cancelled and all routines are closed.
	if s.ConnCancel != nil {
		s.ConnCancel()
		s.sg.WaitAndClear()
	}

	connCtx, connCancel := context.WithCancel(ctx)
	conn, err := s.Dial(connCtx)
	if err != nil {
		return err
	}

	s.Conn = conn
	s.ConnCtx = connCtx
	s.ConnCancel = connCancel

	s.EmitConnect()

	s.sg.Add(func() {
		s.Read(connCtx, conn, connCancel)
	})
	s.sg.Add(func() {
		s.ping(connCtx, conn, connCancel)
	})
	s.sg.Run()
	return nil
}

// Dial opens the websocket connection.
// If an explicit URL argument is provided, it is used. Otherwise, the EndpointCreator
// (if set) is called to construct the URL. If neither is available, an error is returned.
// It also installs default Ping/Pong handlers and logs the mode (public vs user data).
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

	conn, _, err := defaultDialer.DialContext(ctx, url, nil)
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
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout * 2)); err != nil {
			log.WithError(err).Error("pong handler can not set read deadline")
		}
		return nil
	})

	if s.PublicOnly {
		log.Infof("[websocket] public stream connected, read timeout = %v", readTimeout)
	} else {
		log.Infof("[websocket] user data stream connected, read timeout = %v", readTimeout)
	}

	return conn, nil
}

// Close stops the stream gracefully:
// - closes CloseC to stop read/ping workers
// - cancels the connection context
// - writes a websocket close frame
// After Close returns, internal goroutines should be stopped shortly after.
func (s *StandardStream) Close() error {
	if s.PublicOnly {
		log.Debugf("[websocket] closing public stream...")
	} else {
		log.Debugf("[websocket] closing user data stream...")
	}

	// close the close signal channel, so that reader and ping worker will stop
	close(s.CloseC)

	// get the connection object before call the context cancel function
	s.ConnLock.Lock()
	defer s.ConnLock.Unlock()

	// cancel the context so that the ticker loop and listen key updater will be stopped.
	if s.ConnCancel != nil {
		s.ConnCancel()
	}

	// gracefully write the close message to the connection
	if s.Conn != nil {
		err := s.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			return errors.Wrap(err, "websocket write close message error")
		}
	}

	log.Debugf("[websocket] stream closed")

	// let the reader close the connection
	// TODO: use signal channel instead
	<-time.After(time.Second)
	return nil
}

// String returns a human-readable identifier for debugging purposes.
func (s *StandardStream) String() string {
	ss := "StandardStream"
	ss += fmt.Sprintf("(%p)", s)
	return ss
}

// SetHeartBeat installs a custom heartbeat function executed before each ping.
// Return an error from the heartbeat to force the stream to reconnect.
func (s *StandardStream) SetHeartBeat(fn HeartBeat) {
	s.heartBeat = fn
}

// SetBeforeConnect sets a hook invoked by Connect before dialing.
// Use this to refresh credentials, update listen keys, or compute endpoints.
func (s *StandardStream) SetBeforeConnect(fn BeforeConnect) {
	s.beforeConnect = fn
}

type Depth string

const (
	DepthLevelFull   Depth = "FULL"
	DepthLevelMedium Depth = "MEDIUM"
	DepthLevel1      Depth = "1"
	DepthLevel5      Depth = "5"
	DepthLevel10     Depth = "10"
	DepthLevel15     Depth = "15"
	DepthLevel20     Depth = "20"
	DepthLevel50     Depth = "50"
	DepthLevel200    Depth = "200"
	DepthLevel400    Depth = "400"
)

type Speed string

const (
	SpeedHigh   Speed = "HIGH"
	SpeedMedium Speed = "MEDIUM"
	SpeedLow    Speed = "LOW"
)

// SubscribeOptions provides the standard stream options
type SubscribeOptions struct {
	// TODO: change to Interval type later
	Interval Interval `json:"interval,omitempty"`
	Depth    Depth    `json:"depth,omitempty"`
	Speed    Speed    `json:"speed,omitempty"`
}

func (o SubscribeOptions) String() string {
	if len(o.Interval) > 0 {
		return string(o.Interval)
	}

	return string(o.Depth)
}

type Subscription struct {
	Symbol  string           `json:"symbol"`
	Channel Channel          `json:"channel"`
	Options SubscribeOptions `json:"options"`
}
