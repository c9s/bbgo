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

//go:generate mockgen -destination=mocks/stream.go -package=mocks . MarketInfoProvider
type MarketInfoProvider interface {
	GetAllFeeRates(ctx context.Context) (bybitapi.FeeRates, error)
	QueryMarkets(ctx context.Context) (types.MarketMap, error)
}

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	key, secret    string
	marketProvider MarketInfoProvider
	// TODO: update the fee rate at 7:00 am UTC; rotation required.
	symbolFeeDetails map[string]*symbolFeeDetail

	bookEventCallbacks        []func(e BookEvent)
	marketTradeEventCallbacks []func(e []MarketTradeEvent)
	walletEventCallbacks      []func(e []bybitapi.WalletBalances)
	kLineEventCallbacks       []func(e KLineEvent)
	orderEventCallbacks       []func(e []OrderEvent)
	tradeEventCallbacks       []func(e []TradeEvent)
}

func NewStream(key, secret string, marketProvider MarketInfoProvider) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		// pragma: allowlist nextline secret
		key:            key,
		secret:         secret,
		marketProvider: marketProvider,
	}

	stream.SetEndpointCreator(stream.createEndpoint)
	stream.SetParser(stream.parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetHeartBeat(stream.ping)
	stream.SetBeforeConnect(stream.getAllFeeRates)
	stream.OnConnect(stream.handlerConnect)

	stream.OnBookEvent(stream.handleBookEvent)
	stream.OnMarketTradeEvent(stream.handleMarketTradeEvent)
	stream.OnKLineEvent(stream.handleKLineEvent)
	stream.OnWalletEvent(stream.handleWalletEvent)
	stream.OnOrderEvent(stream.handleOrderEvent)
	stream.OnTradeEvent(stream.handleTradeEvent)
	return stream
}

func (s *Stream) syncSubscriptions(opType WsOpType) error {
	if opType != WsOpTypeUnsubscribe && opType != WsOpTypeSubscribe {
		return fmt.Errorf("unexpected subscription type: %v", opType)
	}

	logger := log.WithField("opType", opType)
	lens := len(s.Subscriptions)
	for begin := 0; begin < lens; begin += spotArgsLimit {
		end := begin + spotArgsLimit
		if end > lens {
			end = lens
		}

		topics := []string{}
		for _, subscription := range s.Subscriptions[begin:end] {
			topic, err := s.convertSubscription(subscription)
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
	}

	return nil
}

func (s *Stream) Unsubscribe() {
	// errors are handled in the syncSubscriptions, so they are skipped here.
	_ = s.syncSubscriptions(WsOpTypeUnsubscribe)
	s.Resubscribe(func(old []types.Subscription) (new []types.Subscription, err error) {
		// clear the subscriptions
		return []types.Subscription{}, nil
	})
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

	case []MarketTradeEvent:
		s.EmitMarketTradeEvent(e)

	case []bybitapi.WalletBalances:
		s.EmitWalletEvent(e)

	case *KLineEvent:
		s.EmitKLineEvent(*e)

	case []OrderEvent:
		s.EmitOrderEvent(e)

	case []TradeEvent:
		s.EmitTradeEvent(e)

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
				return nil, fmt.Errorf("failed to unmarshal data into BookEvent: %+v, err: %w", string(e.WebSocketTopicEvent.Data), err)
			}

			book.Type = e.WebSocketTopicEvent.Type
			book.ServerTime = e.WebSocketTopicEvent.Ts.Time()
			return &book, nil

		case TopicTypeMarketTrade:
			// snapshot only
			var trade []MarketTradeEvent
			err = json.Unmarshal(e.WebSocketTopicEvent.Data, &trade)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal data into MarketTradeEvent: %+v, err: %w", string(e.WebSocketTopicEvent.Data), err)
			}

			return trade, nil

		case TopicTypeKLine:
			var kLines []KLine
			err = json.Unmarshal(e.WebSocketTopicEvent.Data, &kLines)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal data into KLine: %+v, err: %w", string(e.WebSocketTopicEvent.Data), err)
			}

			symbol, err := getSymbolFromTopic(e.Topic)
			if err != nil {
				return nil, err
			}

			return &KLineEvent{KLines: kLines, Symbol: symbol, Type: e.WebSocketTopicEvent.Type}, nil

		case TopicTypeWallet:
			var wallets []bybitapi.WalletBalances
			return wallets, json.Unmarshal(e.WebSocketTopicEvent.Data, &wallets)

		case TopicTypeOrder:
			var orders []OrderEvent
			return orders, json.Unmarshal(e.WebSocketTopicEvent.Data, &orders)

		case TopicTypeTrade:
			var trades []TradeEvent
			return trades, json.Unmarshal(e.WebSocketTopicEvent.Data, &trades)

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
		// errors are handled in the syncSubscriptions, so they are skipped here.
		_ = s.syncSubscriptions(WsOpTypeSubscribe)
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
				string(TopicTypeTrade),
			},
		}); err != nil {
			log.WithError(err).Error("failed to send subscription request")
			return
		}
	}
}

func (s *Stream) convertSubscription(sub types.Subscription) (string, error) {
	switch sub.Channel {

	case types.BookChannel:
		depth := types.DepthLevel1
		if len(sub.Options.Depth) > 0 && sub.Options.Depth == types.DepthLevel50 {
			depth = types.DepthLevel50
		}
		return genTopic(TopicTypeOrderBook, depth, sub.Symbol), nil

	case types.MarketTradeChannel:
		return genTopic(TopicTypeMarketTrade, sub.Symbol), nil

	case types.KLineChannel:
		interval, err := toLocalInterval(sub.Options.Interval)
		if err != nil {
			return "", err
		}

		return genTopic(TopicTypeKLine, interval, sub.Symbol), nil

	}

	return "", fmt.Errorf("unsupported stream channel: %s", sub.Channel)
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

func (s *Stream) handleMarketTradeEvent(events []MarketTradeEvent) {
	for _, event := range events {
		trade, err := event.toGlobalTrade()
		if err != nil {
			log.WithError(err).Error("failed to convert to market trade")
			continue
		}

		s.StandardStream.EmitMarketTrade(trade)
	}
}

func (s *Stream) handleWalletEvent(events []bybitapi.WalletBalances) {
	s.StandardStream.EmitBalanceSnapshot(toGlobalBalanceMap(events))
}

func (s *Stream) handleOrderEvent(events []OrderEvent) {
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

func (s *Stream) handleKLineEvent(klineEvent KLineEvent) {
	if klineEvent.Type != DataTypeSnapshot {
		return
	}

	for _, event := range klineEvent.KLines {
		kline, err := event.toGlobalKLine(klineEvent.Symbol)
		if err != nil {
			log.WithError(err).Error("failed to convert to global k line")
			continue
		}

		if kline.Closed {
			s.EmitKLineClosed(kline)
		} else {
			s.EmitKLine(kline)
		}
	}
}

func (s *Stream) handleTradeEvent(events []TradeEvent) {
	for _, event := range events {
		feeRate, found := s.symbolFeeDetails[event.Symbol]
		if !found {
			log.Warnf("unexpected symbol found, fee rate not supported, symbol: %s", event.Symbol)
			continue
		}

		gTrade, err := event.toGlobalTrade(*feeRate)
		if err != nil {
			log.WithError(err).Errorf("unable to convert: %+v", event)
			continue
		}
		s.StandardStream.EmitTradeUpdate(*gTrade)
	}
}

type symbolFeeDetail struct {
	bybitapi.FeeRate

	BaseCoin  string
	QuoteCoin string
}

// getAllFeeRates retrieves all fee rates from the Bybit API and then fetches markets to ensure the base coin and quote coin
// are correct.
func (e *Stream) getAllFeeRates(ctx context.Context) error {
	feeRates, err := e.marketProvider.GetAllFeeRates(ctx)
	if err != nil {
		return fmt.Errorf("failed to call get fee rates: %w", err)
	}

	symbolMap := map[string]*symbolFeeDetail{}
	for _, f := range feeRates.List {
		if _, found := symbolMap[f.Symbol]; !found {
			symbolMap[f.Symbol] = &symbolFeeDetail{FeeRate: f}
		}
	}

	mkts, err := e.marketProvider.QueryMarkets(ctx)
	if err != nil {
		return fmt.Errorf("failed to get markets: %w", err)
	}

	// update base coin, quote coin into symbolFeeDetail
	for _, mkt := range mkts {
		feeRate, found := symbolMap[mkt.Symbol]
		if !found {
			continue
		}

		feeRate.BaseCoin = mkt.BaseCurrency
		feeRate.QuoteCoin = mkt.QuoteCurrency
	}

	// remove trading pairs that are not present in spot market.
	for k, v := range symbolMap {
		if len(v.BaseCoin) == 0 || len(v.QuoteCoin) == 0 {
			log.Debugf("related market not found: %s, skipping the associated trade", k)
			delete(symbolMap, k)
		}
	}

	e.symbolFeeDetails = symbolMap
	return nil
}
