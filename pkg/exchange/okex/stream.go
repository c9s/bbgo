package okex

import (
	"context"
	"fmt"
	"golang.org/x/time/rate"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/types"
)

var (
	tradeLogLimiter = rate.NewLimiter(rate.Every(time.Minute), 1)
)

type WebsocketOp struct {
	Op   WsEventType `json:"op"`
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

	client *okexapi.RestClient

	// public callbacks
	kLineEventCallbacks        []func(candle KLineEvent)
	bookEventCallbacks         []func(book BookEvent)
	accountEventCallbacks      []func(account okexapi.Account)
	orderDetailsEventCallbacks []func(orderDetails []okexapi.OrderDetails)
	marketTradeEventCallbacks  []func(tradeDetail []MarketTradeEvent)
}

func NewStream(client *okexapi.RestClient) *Stream {
	stream := &Stream{
		client:         client,
		StandardStream: types.NewStandardStream(),
	}

	stream.SetParser(parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetEndpointCreator(stream.createEndpoint)

	stream.OnKLineEvent(stream.handleKLineEvent)
	stream.OnBookEvent(stream.handleBookEvent)
	stream.OnAccountEvent(stream.handleAccountEvent)
	stream.OnMarketTradeEvent(stream.handleMarketTradeEvent)
	stream.OnOrderDetailsEvent(stream.handleOrderDetailsEvent)
	stream.OnConnect(stream.handleConnect)
	stream.OnAuth(stream.handleAuth)
	return stream
}

func (s *Stream) syncSubscriptions(opType WsEventType) error {
	if opType != WsEventTypeUnsubscribe && opType != WsEventTypeSubscribe {
		return fmt.Errorf("unexpected subscription type: %v", opType)
	}

	logger := log.WithField("opType", opType)
	var topics []WebsocketSubscription
	for _, subscription := range s.Subscriptions {
		topic, err := convertSubscription(subscription)
		if err != nil {
			logger.WithError(err).Errorf("convert error, subscription: %+v", subscription)
			return err
		}

		topics = append(topics, topic)
	}

	logger.Infof("%s channels: %+v", opType, topics)
	if err := s.Conn.WriteJSON(WebsocketOp{
		Op:   opType,
		Args: topics,
	}); err != nil {
		logger.WithError(err).Error("failed to send request")
		return err
	}

	return nil
}

func (s *Stream) Unsubscribe() {
	// errors are handled in the syncSubscriptions, so they are skipped here.
	_ = s.syncSubscriptions(WsEventTypeUnsubscribe)
	s.Resubscribe(func(old []types.Subscription) (new []types.Subscription, err error) {
		// clear the subscriptions
		return []types.Subscription{}, nil
	})
}

func (s *Stream) handleConnect() {
	if s.PublicOnly {
		var subs []WebsocketSubscription
		for _, subscription := range s.Subscriptions {
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
		err := s.Conn.WriteJSON(WebsocketOp{
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
		sign := okexapi.Sign(payload, s.client.Secret)
		op := WebsocketOp{
			Op: "login",
			Args: []WebsocketLogin{
				{
					Key:        s.client.Key,
					Passphrase: s.client.Passphrase,
					Timestamp:  msTimestamp,
					Sign:       sign,
				},
			},
		}

		log.Infof("sending okex login request")
		err := s.Conn.WriteJSON(op)
		if err != nil {
			log.WithError(err).Errorf("can not send login message")
		}
	}
}

func (s *Stream) handleAuth() {
	var subs = []WebsocketSubscription{
		{Channel: ChannelAccount},
		{Channel: "orders", InstrumentType: string(okexapi.InstrumentTypeSpot)},
	}

	log.Infof("subscribing private channels: %+v", subs)
	err := s.Conn.WriteJSON(WebsocketOp{
		Op:   "subscribe",
		Args: subs,
	})
	if err != nil {
		log.WithError(err).Error("private channel subscribe error")
	}
}

func (s *Stream) handleOrderDetailsEvent(orderDetails []okexapi.OrderDetails) {
	detailTrades, detailOrders := segmentOrderDetails(orderDetails)

	trades, err := toGlobalTrades(detailTrades)
	if err != nil {
		log.WithError(err).Errorf("error converting order details into trades")
	} else {
		for _, trade := range trades {
			s.EmitTradeUpdate(trade)
		}
	}

	orders, err := toGlobalOrders(detailOrders)
	if err != nil {
		log.WithError(err).Errorf("error converting order details into orders")
	} else {
		for _, order := range orders {
			s.EmitOrderUpdate(order)
		}
	}
}

func (s *Stream) handleAccountEvent(account okexapi.Account) {
	balances := toGlobalBalance(&account)
	s.EmitBalanceUpdate(balances)
}

func (s *Stream) handleBookEvent(data BookEvent) {
	book := data.Book()
	switch data.Action {
	case ActionTypeSnapshot:
		s.EmitBookSnapshot(book)
	case ActionTypeUpdate:
		s.EmitBookUpdate(book)
	}
}

func (s *Stream) handleMarketTradeEvent(data []MarketTradeEvent) {
	for _, event := range data {
		trade, err := event.toGlobalTrade()
		if err != nil {
			if tradeLogLimiter.Allow() {
				log.WithError(err).Error("failed to convert to market trade")
			}
			continue
		}

		s.EmitMarketTrade(trade)
	}
}

func (s *Stream) handleKLineEvent(k KLineEvent) {
	for _, event := range k.Events {
		kline := event.ToGlobal(types.Interval(k.Interval), k.Symbol)
		if kline.Closed {
			s.EmitKLineClosed(kline)
		} else {
			s.EmitKLine(kline)
		}
	}
}

func (s *Stream) createEndpoint(ctx context.Context) (string, error) {
	var url string
	if s.PublicOnly {
		url = okexapi.PublicWebSocketURL
	} else {
		url = okexapi.PrivateWebSocketURL
	}
	return url, nil
}

func (s *Stream) dispatchEvent(e interface{}) {
	switch et := e.(type) {
	case *WebSocketEvent:
		if err := et.IsValid(); err != nil {
			log.Errorf("invalid event: %v", err)
			return
		}
		if et.IsAuthenticated() {
			s.EmitAuth()
		}

	case *BookEvent:
		// there's "books" for 400 depth and books5 for 5 depth
		if et.channel != ChannelBook5 {
			s.EmitBookEvent(*et)
		}
		s.EmitBookTickerUpdate(et.BookTicker())
	case *KLineEvent:
		s.EmitKLineEvent(*et)

	case *okexapi.Account:
		s.EmitAccountEvent(*et)

	case []okexapi.OrderDetails:
		s.EmitOrderDetailsEvent(et)

	case []MarketTradeEvent:
		s.EmitMarketTradeEvent(et)

	}
}
