package bitget

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	v2 "github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi/v2"
	"github.com/c9s/bbgo/pkg/types"
)

var (
	pingBytes = []byte("ping")
	pongBytes = []byte("pong")
)

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	key, secret, passphrase   string
	bookEventCallbacks        []func(o BookEvent)
	marketTradeEventCallbacks []func(o MarketTradeEvent)
	KLineEventCallbacks       []func(o KLineEvent)

	accountEventCallbacks []func(e AccountEvent)

	lastCandle map[string]types.KLine
}

func NewStream(key, secret, passphrase string) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		lastCandle:     map[string]types.KLine{},
		key:            key,
		secret:         secret,
		passphrase:     passphrase,
	}

	stream.SetEndpointCreator(stream.createEndpoint)
	stream.SetParser(parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetHeartBeat(stream.ping)
	stream.OnConnect(stream.handlerConnect)

	stream.OnBookEvent(stream.handleBookEvent)
	stream.OnMarketTradeEvent(stream.handleMaretTradeEvent)
	stream.OnKLineEvent(stream.handleKLineEvent)

	stream.OnAuth(stream.handleAuth)
	stream.OnAccountEvent(stream.handleAccountEvent)
	return stream
}

func (s *Stream) syncSubscriptions(opType WsEventType) error {
	if opType != WsEventUnsubscribe && opType != WsEventSubscribe {
		return fmt.Errorf("unexpected subscription type: %v", opType)
	}

	logger := log.WithField("opType", opType)
	args := []WsArg{}
	for _, subscription := range s.Subscriptions {
		arg, err := convertSubscription(subscription)
		if err != nil {
			logger.WithError(err).Errorf("convert error, subscription: %+v", subscription)
			return err
		}

		args = append(args, arg)
	}

	logger.Infof("%s channels: %+v", opType, args)
	if err := s.Conn.WriteJSON(WsOp{
		Op:   opType,
		Args: args,
	}); err != nil {
		logger.WithError(err).Error("failed to send request")
		return err
	}

	return nil
}

func (s *Stream) Unsubscribe() {
	// errors are handled in the syncSubscriptions, so they are skipped here.
	_ = s.syncSubscriptions(WsEventUnsubscribe)
	s.Resubscribe(func(old []types.Subscription) (new []types.Subscription, err error) {
		// clear the subscriptions
		return []types.Subscription{}, nil
	})
}

func (s *Stream) createEndpoint(_ context.Context) (string, error) {
	var url string
	if s.PublicOnly {
		url = bitgetapi.PublicWebSocketURL
	} else {
		url = v2.PrivateWebSocketURL
	}
	return url, nil
}

func (s *Stream) dispatchEvent(event interface{}) {
	switch e := event.(type) {
	case *WsEvent:
		if err := e.IsValid(); err != nil {
			log.Errorf("invalid event: %v", err)
		}
		if e.IsAuthenticated() {
			s.EmitAuth()
		}

	case *BookEvent:
		s.EmitBookEvent(*e)

	case *MarketTradeEvent:
		s.EmitMarketTradeEvent(*e)

	case *KLineEvent:
		s.EmitKLineEvent(*e)

	case *AccountEvent:
		s.EmitAccountEvent(*e)

	case []byte:
		// We only handle the 'pong' case. Others are unexpected.
		if !bytes.Equal(e, pongBytes) {
			log.Errorf("invalid event: %q", e)
		}
	}
}

func (s *Stream) handleAuth() {
	if err := s.Conn.WriteJSON(WsOp{
		Op: WsEventSubscribe,
		Args: []WsArg{
			{
				InstType: instSpV2,
				Channel:  ChannelAccount,
				Coin:     "default", // default all
			},
		},
	}); err != nil {
		log.WithError(err).Error("failed to send subscription request")
		return
	}
}

func (s *Stream) handlerConnect() {
	if s.PublicOnly {
		// errors are handled in the syncSubscriptions, so they are skipped here.
		_ = s.syncSubscriptions(WsEventSubscribe)
	} else {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)

		if err := s.Conn.WriteJSON(WsOp{
			Op: WsEventLogin,
			Args: []WsArg{
				{
					ApiKey:     s.key,
					Passphrase: s.passphrase,
					Timestamp:  timestamp,
					Sign:       bitgetapi.Sign(fmt.Sprintf("%sGET/user/verify", timestamp), s.secret),
				},
			},
		}); err != nil {
			log.WithError(err).Error("failed to auth request")
			return
		}
	}
}

func (s *Stream) handleBookEvent(o BookEvent) {
	for _, book := range o.ToGlobalOrderBooks() {
		switch o.actionType {
		case ActionTypeSnapshot:
			s.EmitBookSnapshot(book)

		case ActionTypeUpdate:
			s.EmitBookUpdate(book)
		}
	}
}

// ping implements the bitget text message of WebSocket PingPong.
func (s *Stream) ping(conn *websocket.Conn) error {
	err := conn.WriteMessage(websocket.TextMessage, pingBytes)
	if err != nil {
		log.WithError(err).Error("ping error", err)
		return nil
	}
	return nil
}

func convertSubscription(sub types.Subscription) (WsArg, error) {
	arg := WsArg{
		// support spot only
		InstType: instSp,
		Channel:  "",
		InstId:   sub.Symbol,
	}

	switch sub.Channel {
	case types.BookChannel:
		arg.Channel = ChannelOrderBook5

		switch sub.Options.Depth {
		case types.DepthLevel15:
			arg.Channel = ChannelOrderBook15
		case types.DepthLevel200:
			log.Warn("*** The subscription events for the order book may return fewer than 200 bids/asks at a depth of 200. ***")
			arg.Channel = ChannelOrderBook
		}
		return arg, nil

	case types.MarketTradeChannel:
		arg.Channel = ChannelTrade
		return arg, nil

	case types.KLineChannel:
		interval, found := toLocalInterval[sub.Options.Interval]
		if !found {
			return WsArg{}, fmt.Errorf("interval %s not supported on KLine subscription", sub.Options.Interval)
		}

		arg.Channel = ChannelType(interval)
		return arg, nil
	}

	return arg, fmt.Errorf("unsupported stream channel: %s", sub.Channel)
}

func parseWebSocketEvent(in []byte) (interface{}, error) {
	switch {
	case bytes.Equal(in, pongBytes):
		// Return the original raw data may seem redundant because we can validate the string and return nil,
		// but we cannot return nil to a lower level handler. This can cause confusion in the next handler, such as
		// the dispatch handler. Therefore, I return the original raw data.
		return in, nil
	default:
		return parseEvent(in)
	}
}

func parseEvent(in []byte) (interface{}, error) {
	var event WsEvent

	err := json.Unmarshal(in, &event)
	if err != nil {
		return nil, err
	}

	if event.IsOp() {
		return &event, nil
	}

	ch := event.Arg.Channel
	switch ch {
	case ChannelAccount:
		var acct AccountEvent
		err = json.Unmarshal(event.Data, &acct.Balances)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal data into AccountEvent, Arg: %+v Data: %s, err: %w", event.Arg, string(event.Data), err)
		}

		acct.actionType = event.Action
		acct.instId = event.Arg.InstId
		return &acct, nil

	case ChannelOrderBook, ChannelOrderBook5, ChannelOrderBook15:
		var book BookEvent
		err = json.Unmarshal(event.Data, &book.Events)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal data into BookEvent, Arg: %+v Data: %s, err: %w", event.Arg, string(event.Data), err)
		}

		book.actionType = event.Action
		book.instId = event.Arg.InstId
		return &book, nil

	case ChannelTrade:
		var trade MarketTradeEvent
		err = json.Unmarshal(event.Data, &trade.Events)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal data into MarketTradeEvent, Arg: %+v Data: %s, err: %w", event.Arg, string(event.Data), err)
		}

		trade.actionType = event.Action
		trade.instId = event.Arg.InstId
		return &trade, nil

	default:

		// handle the `KLine` case here to avoid complicating the code structure.
		if strings.HasPrefix(string(ch), "candle") {
			var kline KLineEvent
			err = json.Unmarshal(event.Data, &kline.Events)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal data into KLineEvent, Arg: %+v Data: %s, err: %w", event.Arg, string(event.Data), err)
			}

			kline.actionType = event.Action
			kline.channel = ch
			kline.instId = event.Arg.InstId
			return &kline, nil
		}
		// return an error for any other case

		return nil, fmt.Errorf("unhandled websocket event: %+v", string(in))
	}
}

func (s *Stream) handleMaretTradeEvent(m MarketTradeEvent) {
	if m.actionType == ActionTypeSnapshot {
		// we don't support snapshot event
		return
	}
	for _, trade := range m.Events {
		globalTrade, err := trade.ToGlobal(m.instId)
		if err != nil {
			log.WithError(err).Error("failed to convert to market trade")
			return
		}

		s.EmitMarketTrade(globalTrade)
	}
}

func (s *Stream) handleKLineEvent(k KLineEvent) {
	if k.actionType == ActionTypeSnapshot {
		// we don't support snapshot event
		return
	}

	interval, found := toGlobalInterval[string(k.channel)]
	if !found {
		log.Errorf("unexpected interval %s on KLine subscription", k.channel)
		return
	}

	for _, kline := range k.Events {
		last, ok := s.lastCandle[k.CacheKey()]
		if ok && kline.StartTime.Time().After(last.StartTime.Time()) {
			last.Closed = true
			s.EmitKLineClosed(last)
		}

		kLine := kline.ToGlobal(interval, k.instId)
		s.EmitKLine(kLine)
		s.lastCandle[k.CacheKey()] = kLine
	}
}

func (s *Stream) handleAccountEvent(m AccountEvent) {
	balanceMap := toGlobalBalanceMap(m.Balances)
	if len(balanceMap) == 0 {
		return
	}

	if m.actionType == ActionTypeUpdate {
		s.StandardStream.EmitBalanceUpdate(balanceMap)
		return
	}
	s.StandardStream.EmitBalanceSnapshot(balanceMap)
}
