package binance

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance"
	"github.com/c9s/bbgo/pkg/bbgo/types"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
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

//go:generate callbackgen -type PrivateStream -interface
type PrivateStream struct {
	Client        *binance.Client
	ListenKey     string
	Conn          *websocket.Conn
	Subscriptions []Subscription

	connectCallbacks []func(stream *PrivateStream)
	tradeCallbacks   []func(trade *types.Trade)

	// custom callbacks
	kLineEventCallbacks       []func(event *KLineEvent)
	kLineClosedEventCallbacks []func(event *KLineEvent)

	balanceUpdateEventCallbacks       []func(event *BalanceUpdateEvent)
	outboundAccountInfoEventCallbacks []func(event *OutboundAccountInfoEvent)
	executionReportEventCallbacks     []func(event *ExecutionReportEvent)
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

	log.Infof("[binance] websocket connected")
	s.Conn = conn

	s.EmitConnect(s)

	var params []string
	for _, subscription := range s.Subscriptions {
		params = append(params, subscription.String())
	}

	log.Infof("[binance] subscribing channels: %+v", params)
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
				log.WithError(err).Error("listen key keep-alive error", err)
			}

		default:
			if err := s.Conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
				log.WithError(err).Errorf("set read deadline error: %s", err.Error())
			}

			mt, message, err := s.Conn.ReadMessage()
			if err != nil {
				log.WithError(err).Errorf("read error: %s", err.Error())
				return
			}

			// skip non-text messages
			if mt != websocket.TextMessage {
				continue
			}

			log.Debugf("[binance] recv: %s", message)

			e, err := ParseEvent(string(message))
			if err != nil {
				log.WithError(err).Errorf("[binance] event parse error")
				continue
			}

			// log.Infof("[binance] event: %+v", e)

			switch e := e.(type) {

			case *OutboundAccountInfoEvent:
				log.Info(e.Event, " ", e.Balances)
				s.EmitOutboundAccountInfoEvent(e)

			case *BalanceUpdateEvent:
				log.Info(e.Event, " ", e.Asset, " ", e.Delta)
				s.EmitBalanceUpdateEvent(e)

			case *KLineEvent:
				log.Info(e.Event, " ", e.KLine, " ", e.KLine.Interval)
				s.EmitKLineEvent(e)

				if e.KLine.Closed {
					s.EmitKLineClosedEvent(e)
				}

			case *ExecutionReportEvent:
				log.Info(e.Event, " ", e)

				s.EmitExecutionReportEvent(e)

				switch e.CurrentExecutionType {
				case "TRADE":
					trade, err := e.Trade()
					if err != nil {
						break
					}
					s.EmitTrade(trade)
				}
			}

			eventC <- e
		}
	}
}

func (s *PrivateStream) Close() error {
	log.Infof("[binance] closing user data stream...")

	defer s.Conn.Close()

	// use background context to close user stream
	err := s.Client.NewCloseUserStreamService().ListenKey(s.ListenKey).Do(context.Background())
	if err != nil {
		log.WithError(err).Error("[binance] error close user data stream")
		return err
	}

	return err
}
