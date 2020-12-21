package binance

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance"
	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/types"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type StreamRequest struct {
	// request ID is required
	ID     int      `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params"`
}

//go:generate callbackgen -type Stream -interface
type Stream struct {
	types.StandardStream

	Client    *binance.Client
	ListenKey string
	Conn      *websocket.Conn

	publicOnly bool

	// custom callbacks
	depthEventCallbacks       []func(e *DepthEvent)
	kLineEventCallbacks       []func(e *KLineEvent)
	kLineClosedEventCallbacks []func(e *KLineEvent)

	balanceUpdateEventCallbacks       []func(event *BalanceUpdateEvent)
	outboundAccountInfoEventCallbacks []func(event *OutboundAccountInfoEvent)
	executionReportEventCallbacks     []func(event *ExecutionReportEvent)
}

func NewStream(client *binance.Client) *Stream {
	// binance BalanceUpdate = withdrawal or deposit changes
	/*
		stream.OnBalanceUpdateEvent(func(e *binance.BalanceUpdateEvent) {
			a.mu.Lock()
			defer a.mu.Unlock()

			delta := util.MustParseFloat(e.Delta)
			if balance, ok := a.Balances[e.Asset]; ok {
				balance.Available += delta
				a.Balances[e.Asset] = balance
			}
		})
	*/

	stream := &Stream{
		Client: client,
	}

	var depthFrames = make(map[string]*DepthFrame)

	stream.OnDepthEvent(func(e *DepthEvent) {
		f, ok := depthFrames[e.Symbol]
		if !ok {
			f = &DepthFrame{
				client: client,
				Symbol: e.Symbol,
			}
			f.OnReady(func(e DepthEvent, bufEvents []DepthEvent) {
				snapshot, err := e.OrderBook()
				if err != nil {
					log.WithError(err).Error("book convert error")
					return
				}
				stream.EmitBookSnapshot(snapshot)

				for _, e := range bufEvents {
					book, err := e.OrderBook()
					if err != nil {
						log.WithError(err).Error("book convert error")
						return
					}

					stream.EmitBookUpdate(book)
				}
			})

			f.OnPush(func(e DepthEvent) {
				book, err := e.OrderBook()
				if err != nil {
					log.WithError(err).Error("book convert error")
					return
				}

				stream.EmitBookUpdate(book)
			})
			depthFrames[e.Symbol] = f
		} else {
			f.PushEvent(*e)
		}
	})

	stream.OnOutboundAccountInfoEvent(func(e *OutboundAccountInfoEvent) {
		snapshot := types.BalanceMap{}
		for _, balance := range e.Balances {
			available := fixedpoint.Must(fixedpoint.NewFromString(balance.Free))
			locked := fixedpoint.Must(fixedpoint.NewFromString(balance.Locked))
			snapshot[balance.Asset] = types.Balance{
				Currency:  balance.Asset,
				Available: available,
				Locked:    locked,
			}
		}
		stream.EmitBalanceSnapshot(snapshot)
	})

	stream.OnKLineEvent(func(e *KLineEvent) {
		kline := e.KLine.KLine()
		if e.KLine.Closed {
			stream.EmitKLineClosedEvent(e)
			stream.EmitKLineClosed(kline)
		} else {
			stream.EmitKLine(kline)
		}
	})

	stream.OnExecutionReportEvent(func(e *ExecutionReportEvent) {
		switch e.CurrentExecutionType {

		case "NEW", "CANCELED", "REJECTED", "EXPIRED", "REPLACED":
			order, err := e.Order()
			if err != nil {
				log.WithError(err).Error("order convert error")
				return
			}

			stream.EmitOrderUpdate(*order)

		case "TRADE":
			trade, err := e.Trade()
			if err != nil {
				log.WithError(err).Error("trade convert error")
				return
			}

			stream.EmitTradeUpdate(*trade)
		}
	})

	stream.OnConnect(func() {
		var params []string
		for _, subscription := range stream.Subscriptions {
			params = append(params, convertSubscription(subscription))
		}

		if len(params) == 0 {
			return
		}

		log.Infof("subscribing channels: %+v", params)
		err := stream.Conn.WriteJSON(StreamRequest{
			Method: "SUBSCRIBE",
			Params: params,
			ID:     1,
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

func (s *Stream) dial(listenKey string) (*websocket.Conn, error) {
	var url string
	if s.publicOnly {
		url = "wss://stream.binance.com:9443/ws"
	} else {
		url = "wss://stream.binance.com:9443/ws/" + listenKey
	}

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *Stream) connect(ctx context.Context) error {
	if s.publicOnly {
		log.Infof("stream is set to public only mode")
	} else {
		log.Infof("creating user data stream...")
		listenKey, err := s.Client.NewStartUserStreamService().Do(ctx)
		if err != nil {
			return err
		}

		s.ListenKey = listenKey
		log.Infof("user data stream created. listenKey: %s", s.ListenKey)
	}

	conn, err := s.dial(s.ListenKey)
	if err != nil {
		return err
	}

	log.Infof("websocket connected")
	s.Conn = conn

	s.EmitConnect()
	return nil
}

func convertSubscription(s types.Subscription) string {
	// binance uses lower case symbol name,
	// for kline, it's "<symbol>@kline_<interval>"
	// for depth, it's "<symbol>@depth OR <symbol>@depth@100ms"
	switch s.Channel {
	case types.KLineChannel:
		return fmt.Sprintf("%s@%s_%s", strings.ToLower(s.Symbol), s.Channel, s.Options.String())

	case types.BookChannel:
		return fmt.Sprintf("%s@depth", strings.ToLower(s.Symbol))
	}

	return fmt.Sprintf("%s@%s", strings.ToLower(s.Symbol), s.Channel)
}

func (s *Stream) Connect(ctx context.Context) error {
	err := s.connect(ctx)
	if err != nil {
		return err
	}

	go s.read(ctx)
	return nil
}

func (s *Stream) read(ctx context.Context) {

	pingTicker := time.NewTicker(1 * time.Minute)
	defer pingTicker.Stop()

	keepAliveTicker := time.NewTicker(5 * time.Minute)
	defer keepAliveTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-keepAliveTicker.C:
			if !s.publicOnly {
				if err := s.Client.NewKeepaliveUserStreamService().ListenKey(s.ListenKey).Do(ctx); err != nil {
					log.WithError(err).Errorf("listen key keep-alive error: %v key: %s", err, maskListenKey(s.ListenKey))
				}
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
						if !s.publicOnly {
							if err := s.invalidateListenKey(ctx, s.ListenKey); err != nil {
								log.WithError(err).Error("invalidate listen key error")
							}
						}

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

			log.Debugf("recv: %s", message)

			e, err := ParseEvent(string(message))
			if err != nil {
				log.WithError(err).Errorf("[binance] event parse error")
				continue
			}

			// log.NotifyTo("[binance] event: %+v", e)
			switch e := e.(type) {

			case *OutboundAccountInfoEvent:
				log.Info(e.Event, " ", e.Balances)
				s.EmitOutboundAccountInfoEvent(e)

			case *BalanceUpdateEvent:
				log.Info(e.Event, " ", e.Asset, " ", e.Delta)
				s.EmitBalanceUpdateEvent(e)

			case *KLineEvent:
				s.EmitKLineEvent(e)

			case *DepthEvent:
				s.EmitDepthEvent(e)

			case *ExecutionReportEvent:
				log.Info(e.Event, " ", e)
				s.EmitExecutionReportEvent(e)
			}
		}
	}
}

func (s *Stream) invalidateListenKey(ctx context.Context, listenKey string) error {
	// use background context to invalidate the user stream
	err := s.Client.NewCloseUserStreamService().ListenKey(listenKey).Do(ctx)
	if err != nil {
		log.WithError(err).Error("[binance] error deleting listen key")
		return err
	}

	return nil
}

func (s *Stream) Close() error {
	log.Infof("closing user data stream...")

	if !s.publicOnly {
		if err := s.invalidateListenKey(context.Background(), s.ListenKey); err != nil {
			log.WithError(err).Error("invalidate listen key error")
		}
		log.Infof("user data stream closed")
	}

	return s.Conn.Close()
}

func maskListenKey(listenKey string) string {
	maskKey := listenKey[0:5]
	return maskKey + strings.Repeat("*", len(listenKey)-1-5)
}

//go:generate callbackgen -type DepthFrame
type DepthFrame struct {
	client *binance.Client

	mu            sync.Mutex
	once          sync.Once
	SnapshotDepth *DepthEvent
	Symbol        string
	BufEvents     []DepthEvent

	readyCallbacks []func(snapshotDepth DepthEvent, bufEvents []DepthEvent)
	pushCallbacks  []func(e DepthEvent)
}

func (f *DepthFrame) Reset() {
	f.mu.Lock()
	f.SnapshotDepth = nil
	f.once = sync.Once{}
	f.mu.Unlock()
}

func (f *DepthFrame) loadDepthSnapshot() {
	depth, err := f.fetch(context.Background())
	if err != nil {
		return
	}

	f.mu.Lock()
	f.SnapshotDepth = depth

	var events []DepthEvent
	for _, e := range f.BufEvents {
		/*
			if i == 0 {
				if e.FirstUpdateID > f.SnapshotDepth.FinalUpdateID+1 {
					// FIXME: we missed some events
					log.Warn("miss matched final update id for order book")
					f.SnapshotDepth = nil
					f.mu.Unlock()
					return
				}
			}
		*/
		if e.FirstUpdateID <= f.SnapshotDepth.FinalUpdateID || e.FinalUpdateID <= f.SnapshotDepth.FinalUpdateID {
			continue
		}

		events = append(events, e)
	}

	f.BufEvents = nil
	f.EmitReady(*depth, events)
	f.mu.Unlock()
}

func (f *DepthFrame) PushEvent(e DepthEvent) {
	f.mu.Lock()
	if f.SnapshotDepth == nil {
		f.BufEvents = append(f.BufEvents, e)
		f.mu.Unlock()

		go f.once.Do(f.loadDepthSnapshot)
		go func() {
			ctx := context.Background()
			ticker := time.NewTicker(5*time.Minute + time.Duration(rand.Intn(10))*time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					f.loadDepthSnapshot()
				}
			}
		}()
	} else {
		if e.FirstUpdateID > f.SnapshotDepth.FinalUpdateID || e.FinalUpdateID > f.SnapshotDepth.FinalUpdateID {
			if e.FinalUpdateID > f.SnapshotDepth.FinalUpdateID {
				f.SnapshotDepth.FinalUpdateID = e.FinalUpdateID
			}

			f.EmitPush(e)
		}
		f.mu.Unlock()
	}
}

// fetch fetches the depth and convert to the depth event so that we can reuse the event structure to convert it to the global orderbook type
func (f *DepthFrame) fetch(ctx context.Context) (*DepthEvent, error) {
	response, err := f.client.NewDepthService().Symbol(f.Symbol).Do(ctx)
	if err != nil {
		return nil, err
	}

	event := DepthEvent{
		FirstUpdateID: 0,
		FinalUpdateID: response.LastUpdateID,
	}

	for _, entry := range response.Bids {
		event.Bids = append(event.Bids, DepthEntry{PriceLevel: entry.Price, Quantity: entry.Quantity})
	}

	for _, entry := range response.Asks {
		event.Asks = append(event.Asks, DepthEntry{PriceLevel: entry.Price, Quantity: entry.Quantity})
	}

	return &event, nil
}
