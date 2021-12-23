package okex

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/gorilla/websocket"
)

const readTimeout = 15 * time.Second

type WebsocketOp struct {
	Op   string      `json:"op"`
	Args interface{} `json:"args"`
}

type WebsocketLogin struct {
	Key        string `json:"apiKey"`
	Passphrase string `json:"passphrase"`
	Timestamp  string `json:"timestamp"`
	Sign       string `json:"sign"`
}

//go:generate callbackgen -type Stream -interface
type Stream struct {
	types.StandardStream

	Client     *okexapi.RestClient
	Conn       *websocket.Conn
	connLock   sync.Mutex
	connCtx    context.Context
	connCancel context.CancelFunc

	// public callbacks
	candleDataCallbacks   []func(candle Candle)
	bookDataCallbacks     []func(book BookData)
	eventCallbacks        []func(event WebSocketEvent)
	accountCallbacks      []func(account okexapi.Account)
	orderDetailsCallbacks []func(orderDetails []okexapi.OrderDetails)

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

	stream.OnAccount(func(account okexapi.Account) {
		balances := toGlobalBalance(&account)
		stream.EmitBalanceSnapshot(balances)
	})

	stream.OnOrderDetails(func(orderDetails []okexapi.OrderDetails) {
		detailTrades, detailOrders := segmentOrderDetails(orderDetails)

		trades, err := toGlobalTrades(detailTrades)
		if err != nil {
			log.WithError(err).Errorf("error converting order details into trades")
		} else {
			for _, trade := range trades {
				stream.EmitTradeUpdate(trade)
			}
		}

		orders, err := toGlobalOrders(detailOrders)
		if err != nil {
			log.WithError(err).Errorf("error converting order details into orders")
		} else {
			for _, order := range orders {
				stream.EmitOrderUpdate(order)
			}
		}
	})

	stream.OnEvent(func(event WebSocketEvent) {
		switch event.Event {
		case "login":
			if event.Code == "0" {
				var subs = []WebsocketSubscription{
					{Channel: "account"},
					{Channel: "orders", InstrumentType: string(okexapi.InstrumentTypeSpot)},
				}

				log.Infof("subscribing private channels: %+v", subs)
				err := stream.Conn.WriteJSON(WebsocketOp{
					Op:   "subscribe",
					Args: subs,
				})

				if err != nil {
					log.WithError(err).Error("private channel subscribe error")
				}
			}
		}
	})

	stream.OnConnect(func() {
		if stream.PublicOnly {
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
		} else {
			// login as private channel
			// sign example:
			// sign=CryptoJS.enc.Base64.Stringify(CryptoJS.HmacSHA256(timestamp +'GET'+'/users/self/verify', secretKey))
			msTimestamp := strconv.FormatFloat(float64(time.Now().UnixNano())/float64(time.Second), 'f', -1, 64)
			payload := msTimestamp + "GET" + "/users/self/verify"
			sign := okexapi.Sign(payload, stream.Client.Secret)
			op := WebsocketOp{
				Op: "login",
				Args: []WebsocketLogin{
					{
						Key:        stream.Client.Key,
						Passphrase: stream.Client.Passphrase,
						Timestamp:  msTimestamp,
						Sign:       sign,
					},
				},
			}

			log.Infof("sending okex login request")
			err := stream.Conn.WriteJSON(op)
			if err != nil {
				log.WithError(err).Errorf("can not send login message")
			}
		}
	})

	return stream
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
	// when in public mode, the listen key is an empty string
	var url string
	if s.PublicOnly {
		url = okexapi.PublicWebSocketURL
	} else {
		url = okexapi.PrivateWebSocketURL
	}

	conn, err := s.StandardStream.Dial(url)
	if err != nil {
		return err
	}

	log.Infof("websocket connected: %s", url)

	// should only start one connection one time, so we lock the mutex
	s.connLock.Lock()

	// ensure the previous context is cancelled
	if s.connCancel != nil {
		s.connCancel()
	}

	// create a new context
	s.connCtx, s.connCancel = context.WithCancel(ctx)

	conn.SetReadDeadline(time.Now().Add(readTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})

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
			s.connLock.Lock()
			conn := s.Conn
			s.connLock.Unlock()

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
					// there's "books" for 400 depth and books5 for 5 depth
					if et.channel != "books5" {
						s.EmitBookData(*et)
					}
					s.EmitBookTickerUpdate(et.BookTicker())
				case *Candle:
					s.EmitCandleData(*et)

				case *okexapi.Account:
					s.EmitAccount(*et)

				case []okexapi.OrderDetails:
					s.EmitOrderDetails(et)

				}
			}
		}
	}
}

func (s *Stream) ping(ctx context.Context) {
	pingTicker := time.NewTicker(readTimeout / 2)
	defer pingTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			log.Debug("ping worker stopped")
			return

		case <-pingTicker.C:
			s.connLock.Lock()
			conn := s.Conn
			s.connLock.Unlock()

			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(3*time.Second)); err != nil {
				log.WithError(err).Error("ping error", err)
				s.Reconnect()
			}
		}
	}
}
