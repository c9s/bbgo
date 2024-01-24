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

func NewStandardStream() StandardStream {
	return StandardStream{
		ReconnectC:   make(chan struct{}, 1),
		CloseC:       make(chan struct{}),
		sg:           NewSyncGroup(),
		pingInterval: pingInterval,
	}
}

func (s *StandardStream) SetPublicOnly() {
	s.PublicOnly = true
}

func (s *StandardStream) GetPublicOnly() bool {
	return s.PublicOnly
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

	// ensure the previous context is cancelled and all routines are closed.
	if s.ConnCancel != nil {
		s.ConnCancel()
		s.sg.WaitAndClear()
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
				log.WithError(err).Errorf("set read deadline error: %s", err.Error())
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
				log.WithError(err).Errorf("websocket event parse error, message: %s", message)
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

func (s *StandardStream) SetPingInterval(interval time.Duration) {
	s.pingInterval = interval
}

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
				log.WithError(err).Error("ping error", err)
				s.Reconnect()
				return
			}
		}
	}
}

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

func (s *StandardStream) Subscribe(channel Channel, symbol string, options SubscribeOptions) {
	s.subLock.Lock()
	defer s.subLock.Unlock()

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

	s.sg.Add(func() {
		s.Read(connCtx, conn, connCancel)
	})
	s.sg.Add(func() {
		s.ping(connCtx, conn, connCancel)
	})
	s.sg.Run()
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
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout * 2)); err != nil {
			log.WithError(err).Error("pong handler can not set read deadline")
		}
		return nil
	})

	log.Infof("[websocket] connected, public = %v, read timeout = %v", s.PublicOnly, readTimeout)
	return conn, nil
}

func (s *StandardStream) Close() error {
	log.Debugf("[websocket] closing stream...")

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

	log.Debugf("[websocket] stream closed")

	// let the reader close the connection
	<-time.After(time.Second)
	return nil
}

// SetHeartBeat sets the custom heart beat implementation if needed
func (s *StandardStream) SetHeartBeat(fn HeartBeat) {
	s.heartBeat = fn
}

// SetBeforeConnect sets the custom hook function before connect
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
