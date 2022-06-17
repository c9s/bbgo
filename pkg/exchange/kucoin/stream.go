package kucoin

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/depth"
	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const readTimeout = 30 * time.Second

//go:generate callbackgen -type Stream -interface
type Stream struct {
	types.StandardStream

	client   *kucoinapi.RestClient
	exchange *Exchange

	bullet                       *kucoinapi.Bullet
	candleEventCallbacks         []func(candle *WebSocketCandleEvent, e *WebSocketEvent)
	orderBookL2EventCallbacks    []func(e *WebSocketOrderBookL2Event)
	tickerEventCallbacks         []func(e *WebSocketTickerEvent)
	accountBalanceEventCallbacks []func(e *WebSocketAccountBalanceEvent)
	privateOrderEventCallbacks   []func(e *WebSocketPrivateOrderEvent)

	lastCandle   map[string]types.KLine
	depthBuffers map[string]*depth.Buffer
}

func NewStream(client *kucoinapi.RestClient, ex *Exchange) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		client:         client,
		exchange:       ex,
		lastCandle:     make(map[string]types.KLine),
		depthBuffers:   make(map[string]*depth.Buffer),
	}

	stream.SetParser(parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetEndpointCreator(stream.getEndpoint)

	stream.OnConnect(stream.handleConnect)
	stream.OnCandleEvent(stream.handleCandleEvent)
	stream.OnOrderBookL2Event(stream.handleOrderBookL2Event)
	stream.OnTickerEvent(stream.handleTickerEvent)
	stream.OnPrivateOrderEvent(stream.handlePrivateOrderEvent)
	stream.OnAccountBalanceEvent(stream.handleAccountBalanceEvent)
	return stream
}

func (s *Stream) handleCandleEvent(candle *WebSocketCandleEvent, e *WebSocketEvent) {
	kline := candle.KLine()
	last, ok := s.lastCandle[e.Topic]
	if ok && kline.StartTime.After(last.StartTime.Time()) || e.Subject == WebSocketSubjectTradeCandlesAdd {
		last.Closed = true
		s.EmitKLineClosed(last)
	}

	s.EmitKLine(kline)
	s.lastCandle[e.Topic] = kline
}

func (s *Stream) handleOrderBookL2Event(e *WebSocketOrderBookL2Event) {
	f, ok := s.depthBuffers[e.Symbol]
	if ok {
		f.AddUpdate(types.SliceOrderBook{
			Symbol: toGlobalSymbol(e.Symbol),
			Bids:   e.Changes.Bids,
			Asks:   e.Changes.Asks,
		}, e.SequenceStart, e.SequenceEnd)
	} else {
		f = depth.NewBuffer(func() (types.SliceOrderBook, int64, error) {
			return s.exchange.QueryDepth(context.Background(), e.Symbol)
		})
		s.depthBuffers[e.Symbol] = f
		f.SetBufferingPeriod(time.Second)
		f.OnReady(func(snapshot types.SliceOrderBook, updates []depth.Update) {
			if valid, err := snapshot.IsValid(); !valid {
				log.Errorf("depth snapshot is invalid, error: %v", err)
				return
			}

			s.EmitBookSnapshot(snapshot)
			for _, u := range updates {
				s.EmitBookUpdate(u.Object)
			}
		})
		f.OnPush(func(update depth.Update) {
			s.EmitBookUpdate(update.Object)
		})
	}
}

func (s *Stream) handleTickerEvent(e *WebSocketTickerEvent) {}

func (s *Stream) handleAccountBalanceEvent(e *WebSocketAccountBalanceEvent) {
	bm := types.BalanceMap{}
	bm[e.Currency] = types.Balance{
		Currency:  e.Currency,
		Available: e.Available,
		Locked:    e.Hold,
	}
	s.StandardStream.EmitBalanceUpdate(bm)
}

func (s *Stream) handlePrivateOrderEvent(e *WebSocketPrivateOrderEvent) {
	if e.Type == "match" {
		s.StandardStream.EmitTradeUpdate(types.Trade{
			OrderID:       hashStringID(e.OrderId),
			ID:            hashStringID(e.TradeId),
			Exchange:      types.ExchangeKucoin,
			Price:         e.MatchPrice,
			Quantity:      e.MatchSize,
			QuoteQuantity: e.MatchPrice.Mul(e.MatchSize),
			Symbol:        toGlobalSymbol(e.Symbol),
			Side:          toGlobalSide(e.Side),
			IsBuyer:       e.Side == "buy",
			IsMaker:       e.Liquidity == "maker",
			Time:          types.Time(e.Ts.Time()),
			Fee:           fixedpoint.Zero, // not supported
			FeeCurrency:   "",              // not supported
		})
	}

	switch e.Type {
	case "open", "match", "filled", "canceled":
		status := types.OrderStatusNew
		if e.Status == "done" {
			if e.FilledSize == e.Size {
				status = types.OrderStatusFilled
			} else {
				status = types.OrderStatusCanceled
			}
		} else if e.Status == "open" {
			if e.FilledSize.Sign() > 0 {
				status = types.OrderStatusPartiallyFilled
			}
		}

		s.StandardStream.EmitOrderUpdate(types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: e.ClientOid,
				Symbol:        toGlobalSymbol(e.Symbol),
				Side:          toGlobalSide(e.Side),
				Type:          toGlobalOrderType(e.OrderType),
				Quantity:      e.Size,
				Price:         e.Price,
			},
			Exchange:         types.ExchangeKucoin,
			OrderID:          hashStringID(e.OrderId),
			UUID:             e.OrderId,
			Status:           status,
			ExecutedQuantity: e.FilledSize,
			IsWorking:        e.Status == "open",
			CreationTime:     types.Time(e.OrderTime.Time()),
			UpdateTime:       types.Time(e.Ts.Time()),
		})

	default:
		log.Warnf("unhandled private order type: %s, payload: %+v", e.Type, e)

	}
}

func (s *Stream) handleConnect() {
	if s.PublicOnly {
		if err := s.sendSubscriptions(); err != nil {
			log.WithError(err).Errorf("subscription error")
			return
		}
	} else {
		id := time.Now().UnixNano() / int64(time.Millisecond)
		cmds := []WebSocketCommand{
			{
				Id:             id,
				Type:           WebSocketMessageTypeSubscribe,
				Topic:          "/spotMarket/tradeOrders",
				PrivateChannel: true,
				Response:       true,
			},
			{
				Id:             id + 1,
				Type:           WebSocketMessageTypeSubscribe,
				Topic:          "/account/balance",
				PrivateChannel: true,
				Response:       true,
			},
		}
		for _, cmd := range cmds {
			if err := s.Conn.WriteJSON(cmd); err != nil {
				log.WithError(err).Errorf("private subscribe write error, cmd: %+v", cmd)
			}
		}
	}
}

func (s *Stream) sendSubscriptions() error {
	cmds, err := convertSubscriptions(s.Subscriptions)
	if err != nil {
		return errors.Wrapf(err, "subscription convert error, subscriptions: %+v", s.Subscriptions)
	}

	for _, cmd := range cmds {
		if err := s.Conn.WriteJSON(cmd); err != nil {
			return errors.Wrapf(err, "subscribe write error, cmd: %+v", cmd)
		}
	}

	return nil
}

// getEndpoint use the PublicOnly flag to check whether we should allocate a public bullet or private bullet
func (s *Stream) getEndpoint(ctx context.Context) (string, error) {
	var bullet *kucoinapi.Bullet
	var err error
	if s.PublicOnly {
		bullet, err = s.client.BulletService.NewGetPublicBulletRequest().Do(ctx)
	} else {
		bullet, err = s.client.BulletService.NewGetPrivateBulletRequest().Do(ctx)
	}

	if err != nil {
		return "", err
	}

	url, err := bullet.URL()
	if err != nil {
		return "", err
	}

	s.bullet = bullet

	log.Debugf("bullet: %+v", bullet)
	return url.String(), nil
}

func (s *Stream) dispatchEvent(event interface{}) {
	e, ok := event.(*WebSocketEvent)
	if !ok {
		return
	}

	if e.Object == nil {
		return
	}

	switch et := e.Object.(type) {

	case *WebSocketTickerEvent:
		s.EmitTickerEvent(et)

	case *WebSocketOrderBookL2Event:
		s.EmitOrderBookL2Event(et)

	case *WebSocketCandleEvent:
		s.EmitCandleEvent(et, e)

	case *WebSocketAccountBalanceEvent:
		s.EmitAccountBalanceEvent(et)

	case *WebSocketPrivateOrderEvent:
		s.EmitPrivateOrderEvent(et)

	default:
		log.Warnf("unhandled event: %+v", et)

	}
}

type WebSocketConnector interface {
	Conn() *websocket.Conn
	Reconnect()
}

func ping(ctx context.Context, w WebSocketConnector, interval time.Duration) {
	log.Infof("starting websocket ping worker with interval %s", interval)

	pingTicker := time.NewTicker(interval)
	defer pingTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			log.Debug("ping worker stopped")
			return

		case <-pingTicker.C:
			conn := w.Conn()

			if err := conn.WriteJSON(WebSocketCommand{
				Id:   util.UnixMilli(),
				Type: "ping",
			}); err != nil {
				log.WithError(err).Error("websocket ping error", err)
				w.Reconnect()
			}

			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(3*time.Second)); err != nil {
				log.WithError(err).Error("ping error", err)
				w.Reconnect()
			}
		}
	}
}
