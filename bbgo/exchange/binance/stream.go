package binance

import (
	"context"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/util"

	"github.com/adshao/go-binance"
	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/bbgo/types"
)

type StreamRequest struct {
	// request ID is required
	ID     int      `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params"`
}

//go:generate callbackgen -type PrivateStream -interface
type PrivateStream struct {
	types.StandardPrivateStream

	Client    *binance.Client
	ListenKey string
	Conn      *websocket.Conn

	connectCallbacks []func(stream *PrivateStream)

	// custom callbacks
	kLineEventCallbacks       []func(e *KLineEvent)
	kLineClosedEventCallbacks []func(e *KLineEvent)

	balanceUpdateEventCallbacks       []func(event *BalanceUpdateEvent)
	outboundAccountInfoEventCallbacks []func(event *OutboundAccountInfoEvent)
	executionReportEventCallbacks     []func(event *ExecutionReportEvent)
}

func (s *PrivateStream) dial(listenKey string) (*websocket.Conn, error) {
	url := "wss://stream.binance.com:9443/ws/" + listenKey
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *PrivateStream) connect(ctx context.Context) error {
	log.Infof("[binance] creating user data stream...")
	listenKey, err := s.Client.NewStartUserStreamService().Do(ctx)
	if err != nil {
		return err
	}

	s.ListenKey = listenKey
	log.Infof("[binance] user data stream created. listenKey: %s", s.ListenKey)

	conn, err := s.dial(s.ListenKey)
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
	return conn.WriteJSON(StreamRequest{
		Method: "SUBSCRIBE",
		Params: params,
		ID:     1,
	})
}

func (s *PrivateStream) Connect(ctx context.Context, eventC chan interface{}) error {
	err := s.connect(ctx)
	if err != nil {
		return err
	}

	go s.read(ctx, eventC)
	return nil
}

func (s *PrivateStream) read(ctx context.Context, eventC chan interface{}) {
	defer close(eventC)

	pingTicker := time.NewTicker(1 * time.Minute)
	defer pingTicker.Stop()

	keepAliveTicker := time.NewTicker(5 * time.Minute)
	defer keepAliveTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-keepAliveTicker.C:
			err := s.Client.NewKeepaliveUserStreamService().ListenKey(s.ListenKey).Do(ctx)
			if err != nil {
				log.WithError(err).Errorf("listen key keep-alive error: %v key: %s", err, maskListenKey(s.ListenKey))
			}

		case <-pingTicker.C:
			if err := s.Conn.WriteControl(websocket.PingMessage, []byte("hb"), time.Now().Add(1*time.Second)); err != nil {
				log.WithError(err).Error("ping error", err)
			}


		default:
			if err := s.Conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
				log.WithError(err).Errorf("set read deadline error: %s", err.Error())
			}

			mt, message, err := s.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
					log.WithError(err).Errorf("read error: %s", err.Error())
				}

				// reconnect
				for err != nil {
					select {
					case <-ctx.Done():
						return

					default:
						_ = s.invalidateListenKey(ctx, s.ListenKey)

						err = s.connect(ctx)
						time.Sleep(5 * time.Second)
					}
				}

				continue
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

			// log.Notify("[binance] event: %+v", e)

			switch e := e.(type) {

			case *OutboundAccountInfoEvent:
				log.Info(e.Event, " ", e.Balances)
				s.EmitOutboundAccountInfoEvent(e)

				snapshot := map[string]types.Balance{}
				for _, balance := range e.Balances {
					available := util.MustParseFloat(balance.Free)
					locked := util.MustParseFloat(balance.Locked)
					snapshot[balance.Asset] = types.Balance{
						Currency:  balance.Asset,
						Available: available,
						Locked:    locked,
					}
				}
				s.EmitBalanceSnapshot(snapshot)

			case *BalanceUpdateEvent:
				log.Info(e.Event, " ", e.Asset, " ", e.Delta)
				s.EmitBalanceUpdateEvent(e)

			case *KLineEvent:
				log.Info(e.Event, " ", e.KLine, " ", e.KLine.Interval)
				s.EmitKLineEvent(e)

				if e.KLine.Closed {
					s.EmitKLineClosedEvent(e)
					s.EmitKLineClosed(e.KLine.KLine())
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

func (s *PrivateStream) invalidateListenKey(ctx context.Context, listenKey string) error {
	// use background context to invalidate the user stream
	err := s.Client.NewCloseUserStreamService().ListenKey(listenKey).Do(ctx)
	if err != nil {
		log.WithError(err).Error("[binance] error deleting listen key")
		return err
	}

	return nil
}

func (s *PrivateStream) Close() error {
	log.Infof("[binance] closing user data stream...")
	defer s.Conn.Close()
	err := s.invalidateListenKey(context.Background(), s.ListenKey)

	log.Infof("[binance] user data stream closed")
	return err
}

func maskListenKey(listenKey string) string {
	maskKey := listenKey[0:5]
	return maskKey + strings.Repeat("*", len(listenKey)-1-5)
}
