package okex

import (
	"context"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/types"
)

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

	client *okexapi.RestClient

	// public callbacks
	candleEventCallbacks       []func(candle Candle)
	bookEventCallbacks         []func(book BookEvent)
	eventCallbacks             []func(event WebSocketEvent)
	accountEventCallbacks      []func(account okexapi.Account)
	orderDetailsEventCallbacks []func(orderDetails []okexapi.OrderDetails)

	lastCandle map[CandleKey]Candle
}

type CandleKey struct {
	InstrumentID string
	Channel      string
}

func NewStream(client *okexapi.RestClient) *Stream {
	stream := &Stream{
		client:         client,
		StandardStream: types.NewStandardStream(),
		lastCandle:     make(map[CandleKey]Candle),
	}

	stream.SetParser(parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetEndpointCreator(stream.createEndpoint)

	stream.OnCandleEvent(stream.handleCandleEvent)
	stream.OnBookEvent(stream.handleBookEvent)
	stream.OnAccountEvent(stream.handleAccountEvent)
	stream.OnOrderDetailsEvent(stream.handleOrderDetailsEvent)
	stream.OnEvent(stream.handleEvent)
	stream.OnConnect(stream.handleConnect)
	return stream
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

func (s *Stream) handleEvent(event WebSocketEvent) {
	switch event.Event {
	case "login":
		if event.Code == "0" {
			var subs = []WebsocketSubscription{
				{Channel: "account"},
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
	s.EmitBalanceSnapshot(balances)
}

func (s *Stream) handleBookEvent(data BookEvent) {
	book := data.Book()
	switch data.Action {
	case "snapshot":
		s.EmitBookSnapshot(book)
	case "update":
		s.EmitBookUpdate(book)
	}
}

func (s *Stream) handleCandleEvent(candle Candle) {
	key := CandleKey{Channel: candle.Channel, InstrumentID: candle.InstrumentID}
	kline := candle.KLine()

	// check if we need to close previous kline
	lastCandle, ok := s.lastCandle[key]
	if ok && candle.StartTime.After(lastCandle.StartTime) {
		lastKline := lastCandle.KLine()
		lastKline.Closed = true
		s.EmitKLineClosed(lastKline)
	}

	s.EmitKLine(kline)
	s.lastCandle[key] = candle
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
		s.EmitEvent(*et)

	case *BookEvent:
		// there's "books" for 400 depth and books5 for 5 depth
		if et.channel != "books5" {
			s.EmitBookEvent(*et)
		}
		s.EmitBookTickerUpdate(et.BookTicker())
	case *Candle:
		s.EmitCandleEvent(*et)

	case *okexapi.Account:
		s.EmitAccountEvent(*et)

	case []okexapi.OrderDetails:
		s.EmitOrderDetailsEvent(et)

	}
}
