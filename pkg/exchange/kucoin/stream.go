package kucoin

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
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

	Client     *kucoinapi.RestClient
	Conn       *websocket.Conn
	connLock   sync.Mutex
	connCtx    context.Context
	connCancel context.CancelFunc

	publicOnly bool
}

type WebsocketSubscription struct{}

func NewStream(client *kucoinapi.RestClient) *Stream {
	stream := &Stream{
		Client: client,
		StandardStream: types.StandardStream{
			ReconnectC: make(chan struct{}, 1),
		},
	}

	stream.OnConnect(func() {
		if stream.publicOnly {
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
			/*
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

				log.Infof("sending login request: %+v", op)
				err := stream.Conn.WriteJSON(op)
				if err != nil {
					log.WithError(err).Errorf("can not send login message")
				}
			*/
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
			log.Warnf("received reconnect signal, reconnecting...")
			time.Sleep(3 * time.Second)

			if err := s.connect(ctx); err != nil {
				log.WithError(err).Errorf("connect error, try to reconnect again...")
				s.Reconnect()
			}
		}
	}
}

// getEndpoint use the publicOnly flag to check whether we should allocate a public bullet or private bullet
func (s *Stream) getEndpoint() (string, error) {
	var bullet *kucoinapi.Bullet
	var err error
	if s.publicOnly {
		bullet, err = s.Client.BulletService.NewGetPublicBulletRequest().Do(nil)
	} else {
		bullet, err = s.Client.BulletService.NewGetPrivateBulletRequest().Do(nil)
	}

	if err != nil {
		return "", err
	}

	url, err := bullet.URL()
	if err != nil {
		return "", err
	}

	return url.String(), nil
}

func (s *Stream) connect(ctx context.Context) error {
	url, err := s.getEndpoint()
	if err != nil {
		return err
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

			e, err := parseWebsocketPayload(string(message))
			if err != nil {
				log.WithError(err).Error("message parse error")
			}

			if e != nil {
				switch et := e.(type) {

				/*
					case *AccountEvent:
						s.EmitOrderDetails(et)
				*/
				default:
					log.Warnf("unhandled event: %+v", et)

				}
			}
		}
	}
}

func (s *Stream) getConn() *websocket.Conn {
	s.connLock.Lock()
	conn := s.Conn
	s.connLock.Unlock()
	return conn
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
			conn := s.getConn()
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(3*time.Second)); err != nil {
				log.WithError(err).Error("ping error", err)
				s.Reconnect()
			}
		}
	}
}

func parseWebsocketPayload(in string) (interface{}, error) {
	return nil, nil
}
