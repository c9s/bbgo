package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	// Bybit: To avoid network or program issues, we recommend that you send the ping heartbeat packet every 20 seconds
	// to maintain the WebSocket connection.
	pingInterval = 20 * time.Second

	// spotArgsLimit can input up to 10 args for each subscription request sent to one connection.
	spotArgsLimit = 10
)

var (
	// wsAuthRequest specifies the duration for which a websocket request's authentication is valid.
	wsAuthRequest = 10 * time.Second
)

//go:generate callbackgen -type Stream
type Stream struct {
	key, secret string
	types.StandardStream

	bookEventCallbacks   []func(e BookEvent)
	walletEventCallbacks []func(e []*WalletEvent)
	orderEventCallbacks  []func(e []*OrderEvent)
}

func NewStream(key, secret string) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		// pragma: allowlist nextline secret
		key:    key,
		secret: secret,
	}

	stream.SetEndpointCreator(stream.createEndpoint)
	stream.SetParser(stream.parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetHeartBeat(stream.ping)

	stream.OnConnect(stream.handlerConnect)
	stream.OnBookEvent(stream.handleBookEvent)
	stream.OnWalletEvent(stream.handleWalletEvent)
	stream.OnOrderEvent(stream.handleOrderEvent)
	return stream
}

func (s *Stream) createEndpoint(_ context.Context) (string, error) {
	var url string
	if s.PublicOnly {
		url = bybitapi.WsSpotPublicSpotUrl
	} else {
		url = bybitapi.WsSpotPrivateUrl
	}
	return url, nil
}

func (s *Stream) dispatchEvent(event interface{}) {
	switch e := event.(type) {
	case *WebSocketOpEvent:
		if err := e.IsValid(); err != nil {
			log.Errorf("invalid event: %v", err)
		}

	case *BookEvent:
		s.EmitBookEvent(*e)

	case []*WalletEvent:
		s.EmitWalletEvent(e)

	case []*OrderEvent:
		s.EmitOrderEvent(e)
	}
}

func (s *Stream) parseWebSocketEvent(in []byte) (interface{}, error) {
	var e WsEvent

	err := json.Unmarshal(in, &e)
	if err != nil {
		return nil, err
	}

	switch {
	case e.IsOp():
		return e.WebSocketOpEvent, nil

	case e.IsTopic():
		switch getTopicType(e.Topic) {
		case TopicTypeOrderBook:
			var book BookEvent
			err = json.Unmarshal(e.WebSocketTopicEvent.Data, &book)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal data into BookEvent: %+v, : %w", string(e.WebSocketTopicEvent.Data), err)
			}

			book.Type = e.WebSocketTopicEvent.Type
			return &book, nil

		case TopicTypeWallet:
			var wallets []*WalletEvent
			return wallets, json.Unmarshal(e.WebSocketTopicEvent.Data, &wallets)

		case TopicTypeOrder:
			var orders []*OrderEvent
			return orders, json.Unmarshal(e.WebSocketTopicEvent.Data, &orders)

		}
	}

	return nil, fmt.Errorf("unhandled websocket event: %+v", string(in))
}

// ping implements the Bybit text message of WebSocket PingPong.
func (s *Stream) ping(ctx context.Context, conn *websocket.Conn, cancelFunc context.CancelFunc) {
	defer func() {
		log.Debug("[bybit] ping worker stopped")
		cancelFunc()
	}()

	var pingTicker = time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-s.CloseC:
			return

		case <-pingTicker.C:
			// it's just for maintaining the liveliness of the connection, so comment out ReqId.
			err := conn.WriteJSON(struct {
				//ReqId string `json:"req_id"`
				Op WsOpType `json:"op"`
			}{
				//ReqId: uuid.NewString(),
				Op: WsOpTypePing,
			})
			if err != nil {
				log.WithError(err).Error("ping error", err)
				s.Reconnect()
				return
			}
		}
	}
}

func (s *Stream) handlerConnect() {
	if s.PublicOnly {
		var topics []string

		for _, subscription := range s.Subscriptions {
			topic, err := convertSubscription(subscription)
			if err != nil {
				log.WithError(err).Errorf("subscription convert error")
				continue
			}

			topics = append(topics, topic)
		}
		if len(topics) > spotArgsLimit {
			log.Debugf("topics exceeds limit: %d, drop of: %v", spotArgsLimit, topics[spotArgsLimit:])
			topics = topics[:spotArgsLimit]
		}
		log.Infof("subscribing channels: %+v", topics)
		if err := s.Conn.WriteJSON(WebsocketOp{
			Op:   WsOpTypeSubscribe,
			Args: topics,
		}); err != nil {
			log.WithError(err).Error("failed to send subscription request")
			return
		}
	} else {
		expires := strconv.FormatInt(time.Now().Add(wsAuthRequest).In(time.UTC).UnixMilli(), 10)

		if err := s.Conn.WriteJSON(WebsocketOp{
			Op: WsOpTypeAuth,
			Args: []string{
				s.key,
				expires,
				bybitapi.Sign(fmt.Sprintf("GET/realtime%s", expires), s.secret),
			},
		}); err != nil {
			log.WithError(err).Error("failed to auth request")
			return
		}

		if err := s.Conn.WriteJSON(WebsocketOp{
			Op: WsOpTypeSubscribe,
			Args: []string{
				string(TopicTypeWallet),
				string(TopicTypeOrder),
			},
		}); err != nil {
			log.WithError(err).Error("failed to send subscription request")
			return
		}
	}
}

func convertSubscription(s types.Subscription) (string, error) {
	switch s.Channel {
	case types.BookChannel:
		depth := types.DepthLevel1
		if len(s.Options.Depth) > 0 && s.Options.Depth == types.DepthLevel50 {
			depth = types.DepthLevel50
		}
		return genTopic(TopicTypeOrderBook, depth, s.Symbol), nil
	}

	return "", fmt.Errorf("unsupported stream channel: %s", s.Channel)
}

func (s *Stream) handleBookEvent(e BookEvent) {
	orderBook := e.OrderBook()
	switch {
	// Occasionally, you'll receive "UpdateId"=1, which is a snapshot data due to the restart of
	// the service. So please overwrite your local orderbook
	case e.Type == DataTypeSnapshot || e.UpdateId.Int() == 1:
		s.EmitBookSnapshot(orderBook)

	case e.Type == DataTypeDelta:
		s.EmitBookUpdate(orderBook)
	}
}

func (s *Stream) handleWalletEvent(events []*WalletEvent) {
	bm := types.BalanceMap{}
	for _, event := range events {
		if event.AccountType != AccountTypeSpot {
			return
		}

		for _, obj := range event.Coins {
			bm[obj.Coin] = types.Balance{
				Currency:  obj.Coin,
				Available: obj.Free,
				Locked:    obj.Locked,
			}
		}
	}

	s.StandardStream.EmitBalanceSnapshot(bm)
}

func (s *Stream) handleOrderEvent(events []*OrderEvent) {
	for _, event := range events {
		if event.Category != bybitapi.CategorySpot {
			return
		}

		gOrder, err := toGlobalOrder(event.Order)
		if err != nil {
			log.WithError(err).Error("failed to convert to global order")
			continue
		}
		s.StandardStream.EmitOrderUpdate(*gOrder)
	}
}
