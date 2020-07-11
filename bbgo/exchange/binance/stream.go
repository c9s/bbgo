package binance

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

type SubscribeOptions struct {
	Interval string
	Depth    string
}

func (o SubscribeOptions) String() string {
	if len(o.Interval) > 0 {
		return o.Interval
	}

	return o.Depth
}

type Subscription struct {
	Symbol  string
	Channel string
	Options SubscribeOptions
}

func (s *Subscription) String() string {
	// binance uses lower case symbol name
	return fmt.Sprintf("%s@%s_%s", strings.ToLower(s.Symbol), s.Channel, s.Options.String())
}

type StreamRequest struct {
	// request ID is required
	ID     int      `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params"`
}

type PrivateStream struct {
	Client        *binance.Client
	ListenKey     string
	Conn          *websocket.Conn
	Subscriptions []Subscription
}

func (s *PrivateStream) Subscribe(channel string, symbol string, options SubscribeOptions) {
	s.Subscriptions = append(s.Subscriptions, Subscription{
		Channel: channel,
		Symbol:  symbol,
		Options: options,
	})
}

func (s *PrivateStream) Connect(ctx context.Context, eventC chan interface{}) error {
	url := "wss://stream.binance.com:9443/ws/" + s.ListenKey
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	logrus.Infof("[binance] websocket connected")
	s.Conn = conn

	var params []string
	for _, subscription := range s.Subscriptions {
		params = append(params, subscription.String())
	}

	logrus.Infof("[binance] subscribing channels: %+v", params)
	err = conn.WriteJSON(StreamRequest{
		Method: "SUBSCRIBE",
		Params: params,
		ID:     1,
	})

	if err != nil {
		return err
	}

	go s.read(ctx, eventC)

	return nil
}

func (s *PrivateStream) read(ctx context.Context, eventC chan interface{}) {
	defer close(eventC)

	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-ticker.C:
			err := s.Client.NewKeepaliveUserStreamService().ListenKey(s.ListenKey).Do(ctx)
			if err != nil {
				logrus.WithError(err).Error("listen key keep-alive error", err)
			}

		default:
			if err := s.Conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
				logrus.WithError(err).Errorf("set read deadline error: %s", err.Error())
			}

			mt, message, err := s.Conn.ReadMessage()
			if err != nil {
				logrus.WithError(err).Errorf("read error: %s", err.Error())
				return
			}

			// skip non-text messages
			if mt != websocket.TextMessage {
				continue
			}

			logrus.Debugf("[binance] recv: %s", message)

			e, err := ParseEvent(string(message))
			if err != nil {
				logrus.WithError(err).Errorf("[binance] event parse error")
				continue
			}

			eventC <- e
		}
	}
}

func (s *PrivateStream) Close() error {
	logrus.Infof("[binance] closing user data stream...")

	defer s.Conn.Close()

	// use background context to close user stream
	err := s.Client.NewCloseUserStreamService().ListenKey(s.ListenKey).Do(context.Background())
	if err != nil {
		logrus.WithError(err).Error("[binance] error close user data stream")
		return err
	}

	return err
}

