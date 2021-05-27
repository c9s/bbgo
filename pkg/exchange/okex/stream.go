package okex

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/gorilla/websocket"
)

type WebsocketOp struct {
	Op   string      `json:"op"`
	Args interface{} `json:"args"`
}

//go:generate callbackgen -type Stream -interface
type Stream struct {
	types.StandardStream

	Client     *okexapi.RestClient
	Conn       *websocket.Conn
	connLock   sync.Mutex
	connCtx    context.Context
	connCancel context.CancelFunc

	publicOnly bool

	// public callbacks
	candleDataCallbacks []func(candle Candle)
	bookDataCallbacks   []func(book BookData)
	eventCallbacks      []func(event WebSocketEvent)

	lastCandle map[CandleKey]Candle
}

type CandleKey struct {
	InstrumentID string
	Channel      string
}

func NewStream(client *okexapi.RestClient) *Stream {
	stream := &Stream{
		Client: client,
		StandardStream: types.StandardStream{
			ReconnectC: make(chan struct{}, 1),
		},
		lastCandle: make(map[CandleKey]Candle),
	}

	stream.OnCandleData(func(candle Candle) {
		key := CandleKey{Channel: candle.Channel, InstrumentID: candle.InstrumentID}
		kline := candle.KLine()

		// check if we need to close previous kline
		lastCandle, ok := stream.lastCandle[key]
		if ok && candle.StartTime.After(lastCandle.StartTime) {
			lastKline := lastCandle.KLine()
			lastKline.Closed = true
			stream.EmitKLineClosed(lastKline)
		}

		stream.EmitKLine(kline)
		stream.lastCandle[key] = candle
	})

	stream.OnBookData(func(data BookData) {
		book := data.Book()
		switch data.Action {
		case "snapshot":
			stream.EmitBookSnapshot(book)
		case "update":
			stream.EmitBookUpdate(book)
		}
	})

	stream.OnConnect(func() {
		var subs []WebsocketSubscription
		for _, subscription := range stream.Subscriptions {
			sub, err := convertSubscription(subscription)
			if err != nil {
				log.WithError(err).Errorf("subscription convert error")
				continue
			}

			subs = append(subs, sub)
		}

		if len(subs) == 0 {
			return
		}

		log.Infof("subscribing channels: %+v", subs)
		err := stream.Conn.WriteJSON(WebsocketOp{
			Op:   "subscribe",
			Args: subs,
		})

		if err != nil {
			log.WithError(err).Error("subscribe error")
		}
	})

	return stream
}

func (s *Stream) SetPublicOnly() {
	s.publicOnly = true
}

func (s *Stream) Close() error {
	return nil
}

func (s *Stream) Connect(ctx context.Context) error {
	err := s.connect(ctx)
	if err != nil {
		return err
	}

	// start one re-connector goroutine with the base context
	go s.Reconnector(ctx)

	s.EmitStart()
	return nil
}

func (s *Stream) Reconnector(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-s.ReconnectC:
			// ensure the previous context is cancelled
			if s.connCancel != nil {
				s.connCancel()
			}

			log.Warnf("received reconnect signal, reconnecting...")
			time.Sleep(3 * time.Second)

			if err := s.connect(ctx); err != nil {
				log.WithError(err).Errorf("connect error, try to reconnect again...")
				s.Reconnect()
			}
		}
	}
}

func (s *Stream) connect(ctx context.Context) error {
	// should only start one connection one time, so we lock the mutex
	s.connLock.Lock()

	// create a new context
	s.connCtx, s.connCancel = context.WithCancel(ctx)

	if s.publicOnly {
		log.Infof("stream is set to public only mode")
	} else {
		log.Infof("request listen key for creating user data stream...")
	}

	// when in public mode, the listen key is an empty string
	var url string
	if s.publicOnly {
		url = okexapi.PublicWebSocketURL
	} else {
		url = okexapi.PrivateWebSocketURL
	}

	conn, err := s.StandardStream.Dial(url)
	if err != nil {
		s.connCancel()
		s.connLock.Unlock()
		return err
	}

	log.Infof("websocket connected")

	s.Conn = conn
	s.connLock.Unlock()

	s.EmitConnect()

	go s.read(s.connCtx)
	go s.ping(s.connCtx)
	return nil
}

func (s *Stream) read(ctx context.Context) {
	defer func() {
		if s.connCancel != nil {
			s.connCancel()
		}
		s.EmitDisconnect()
	}()

	for {
		select {

		case <-ctx.Done():
			return

		default:
			if err := s.Conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
				log.WithError(err).Errorf("set read deadline error: %s", err.Error())
			}

			mt, message, err := s.Conn.ReadMessage()

			if err != nil {
				// if it's a network timeout error, we should re-connect
				switch err := err.(type) {

				// if it's a websocket related error
				case *websocket.CloseError:
					if err.Code == websocket.CloseNormalClosure {
						return
					}

					// for unexpected close error, we should re-connect
					// emit reconnect to start a new connection
					s.Reconnect()
					return

				case net.Error:
					log.WithError(err).Error("network error")
					s.Reconnect()
					return

				default:
					log.WithError(err).Error("unexpected connection error")
					s.Reconnect()
					return
				}
			}

			// skip non-text messages
			if mt != websocket.TextMessage {
				continue
			}

			e, err := Parse(string(message))
			if err != nil {
				log.WithError(err).Error("message parse error")
			}

			if e != nil {
				switch et := e.(type) {
				case *WebSocketEvent:
					s.EmitEvent(*et)

				case *BookData:
					s.EmitBookData(*et)

				case *Candle:
					s.EmitCandleData(*et)
				}
			}
		}
	}
}

func (s *Stream) ping(ctx context.Context) {
	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			log.Info("ping worker stopped")
			return

		case <-pingTicker.C:
			s.connLock.Lock()
			if err := s.Conn.WriteControl(websocket.PingMessage, []byte("hb"), time.Now().Add(3*time.Second)); err != nil {
				log.WithError(err).Error("ping error", err)
				s.Reconnect()
			}
			s.connLock.Unlock()
		}
	}
}
