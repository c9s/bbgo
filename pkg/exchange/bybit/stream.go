package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	// spotArgsLimit can input up to 10 args for each subscription request sent to one connection.
	spotArgsLimit = 10
)

var (
	// wsAuthRequest specifies the duration for which a websocket request's authentication is valid.
	wsAuthRequest = 10 * time.Second
	// The default taker/maker fees can help us in estimating trading fees in the SPOT market, because trade fees are not
	// provided for traditional accounts on Bybit.
	// https://www.bybit.com/en-US/help-center/article/Trading-Fee-Structure
	defaultTakerFee = fixedpoint.NewFromFloat(0.001)
	defaultMakerFee = fixedpoint.NewFromFloat(0.001)

	marketTradeLogLimiter = rate.NewLimiter(rate.Every(time.Minute), 1)
	tradeLogLimiter       = rate.NewLimiter(rate.Every(time.Minute), 1)
	orderLogLimiter       = rate.NewLimiter(rate.Every(time.Minute), 1)
	kLineLogLimiter       = rate.NewLimiter(rate.Every(time.Minute), 1)
)

// MarketInfoProvider calculates trade fees since trading fees are not supported by streaming.
type MarketInfoProvider interface {
	GetAllFeeRates(ctx context.Context) (bybitapi.FeeRates, error)
	QueryMarkets(ctx context.Context) (types.MarketMap, error)
}

// AccountBalanceProvider provides a function to query all balances at streaming connected and emit balance snapshot.
type AccountBalanceProvider interface {
	QueryAccountBalances(ctx context.Context) (types.BalanceMap, error)
}

//go:generate mockgen -destination=mocks/stream.go -package=mocks . StreamDataProvider
type StreamDataProvider interface {
	MarketInfoProvider
	AccountBalanceProvider
}

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	key, secret        string
	streamDataProvider StreamDataProvider
	feeRateProvider    *feeRatePoller
	marketsInfo        types.MarketMap

	bookEventCallbacks        []func(e BookEvent)
	marketTradeEventCallbacks []func(e []MarketTradeEvent)
	walletEventCallbacks      []func(e []bybitapi.WalletBalances)
	kLineEventCallbacks       []func(e KLineEvent)
	orderEventCallbacks       []func(e []OrderEvent)
	tradeEventCallbacks       []func(e []TradeEvent)
}

func NewStream(key, secret string, userDataProvider StreamDataProvider) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		// pragma: allowlist nextline secret
		key:                key,
		secret:             secret,
		streamDataProvider: userDataProvider,
		feeRateProvider:    newFeeRatePoller(userDataProvider),
	}

	stream.SetEndpointCreator(stream.createEndpoint)
	stream.SetParser(stream.parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetHeartBeat(stream.ping)
	stream.SetBeforeConnect(func(ctx context.Context) (err error) {
		if stream.PublicOnly {
			// we don't need the fee rate in the public stream.
			return
		}

		// get account fee rate
		go stream.feeRateProvider.Start(ctx)

		stream.marketsInfo, err = stream.streamDataProvider.QueryMarkets(ctx)
		if err != nil {
			log.WithError(err).Error("failed to query market info before to connect stream")
			return err
		}
		return nil
	})
	stream.OnConnect(stream.handlerConnect)
	stream.OnAuth(stream.handleAuthEvent)

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
		if e.IsAuthenticated() {
			s.EmitAuth()
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
		if err = e.IsValid(); err != nil {
			log.Errorf("invalid event: %+v, err: %s", e, err)
			return nil, err
		}

		// return global pong event to avoid emit raw message
		if ok, pongEvent := e.toGlobalPongEventIfValid(); ok {
			return pongEvent, nil
		}
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
func (s *Stream) ping(conn *websocket.Conn) error {
	err := conn.WriteJSON(struct {
		Op WsOpType `json:"op"`
	}{
		Op: WsOpTypePing,
	})
	if err != nil {
		log.WithError(err).Error("ping error")
		return err
	}

	return nil
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

		switch sub.Options.Depth {
		case types.DepthLevel50:
			depth = sub.Options.Depth
		case types.DepthLevel200:
			depth = sub.Options.Depth
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

func (s *Stream) handleAuthEvent() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var balnacesMap types.BalanceMap
	var err error
	err = retry.GeneralBackoff(ctx, func() error {
		balnacesMap, err = s.streamDataProvider.QueryAccountBalances(ctx)
		return err
	})
	if err != nil {
		log.WithError(err).Error("no more attempts to retrieve balances")
		return
	}

	s.EmitBalanceSnapshot(balnacesMap)
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
			if marketTradeLogLimiter.Allow() {
				log.WithError(err).Error("failed to convert to market trade")
			}
			continue
		}

		s.StandardStream.EmitMarketTrade(trade)
	}
}

func (s *Stream) handleWalletEvent(events []bybitapi.WalletBalances) {
	s.StandardStream.EmitBalanceUpdate(toGlobalBalanceMap(events))
}

func (s *Stream) handleOrderEvent(events []OrderEvent) {
	for _, event := range events {
		if event.Category != bybitapi.CategorySpot {
			return
		}

		gOrder, err := toGlobalOrder(event.Order)
		if err != nil {
			if orderLogLimiter.Allow() {
				log.WithError(err).Error("failed to convert to global order")
			}
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
			if kLineLogLimiter.Allow() {
				log.WithError(err).Error("failed to convert to global k line")
			}
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
		feeRate, found := s.feeRateProvider.Get(event.Symbol)
		if !found {
			feeRate = symbolFeeDetail{
				FeeRate: bybitapi.FeeRate{
					Symbol:       event.Symbol,
					TakerFeeRate: defaultTakerFee,
					MakerFeeRate: defaultMakerFee,
				},
				BaseCoin:  "",
				QuoteCoin: "",
			}

			if market, ok := s.marketsInfo[event.Symbol]; ok {
				feeRate.BaseCoin = market.BaseCurrency
				feeRate.QuoteCoin = market.QuoteCurrency
			}

			if tradeLogLimiter.Allow() {
				// The error log level was utilized due to a detected discrepancy in the fee calculations.
				log.Errorf("failed to get %s fee rate, use default taker fee %f, maker fee %f, base coin: %s, quote coin: %s",
					event.Symbol,
					feeRate.TakerFeeRate.Float64(),
					feeRate.MakerFeeRate.Float64(),
					feeRate.BaseCoin,
					feeRate.QuoteCoin,
				)
			}
		}

		gTrade, err := event.toGlobalTrade(feeRate)
		if err != nil {
			if tradeLogLimiter.Allow() {
				log.WithError(err).Errorf("unable to convert: %+v", event)
			}
			continue
		}
		s.StandardStream.EmitTradeUpdate(*gTrade)
	}
}
