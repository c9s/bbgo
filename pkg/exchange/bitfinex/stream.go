package bitfinex

import (
	"context"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/depth"
	bfxapi "github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("exchange", "bitfinex")

// Stream represents the Bitfinex websocket stream.
//
//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	depthBuffers map[string]*depth.Buffer

	responseCallbacks  []func(resp *bfxapi.WebSocketResponse)
	heartBeatCallbacks []func(e *bfxapi.HeartBeatEvent)

	tickerEventCallbacks         []func(e *bfxapi.TickerEvent)
	candleSnapshotEventCallbacks []func(e *bfxapi.CandleSnapshotEvent)
	candleEventCallbacks         []func(e *bfxapi.CandleEvent)

	statusEventCallbacks []func(e *bfxapi.StatusEvent)

	publicTradeEventCallbacks         []func(e *bfxapi.PublicTradeEvent)
	publicTradeSnapshotEventCallbacks []func(e *bfxapi.PublicTradeSnapshotEvent)

	publicFundingTradeEventCallbacks         []func(e *bfxapi.PublicFundingTradeEvent)
	publicFundingTradeSnapshotEventCallbacks []func(e *bfxapi.PublicFundingTradeSnapshotEvent)

	bookUpdateEventCallbacks   []func(e *bfxapi.BookUpdateEvent)
	bookSnapshotEventCallbacks []func(e *bfxapi.BookSnapshotEvent)

	fundingBookEventCallbacks         []func(e *bfxapi.FundingBookUpdateEvent)
	fundingBookSnapshotEventCallbacks []func(e *bfxapi.FundingBookSnapshotEvent)

	walletSnapshotEventCallbacks   []func(e *bfxapi.WalletSnapshotEvent)
	walletUpdateEventCallbacks     []func(e *bfxapi.Wallet)
	positionSnapshotEventCallbacks []func(e *bfxapi.UserPositionSnapshotEvent)
	positionUpdateEventCallbacks   []func(e *bfxapi.UserPosition)

	orderSnapshotEventCallbacks []func(e *bfxapi.UserOrderSnapshotEvent)
	orderUpdateEventCallbacks   []func(e *bfxapi.UserOrder)
	tradeUpdateEventCallbacks   []func(e *bfxapi.TradeUpdateEvent)

	// balanceUpdateEventCallbacks (balance update is actually AUM update in Bitfinex, not the "balance" we usually think of)
	balanceUpdateEventCallbacks []func(e *bfxapi.BalanceUpdateEvent)

	parser *bfxapi.Parser
	logger logrus.FieldLogger

	ex *Exchange
}

// NewStream creates a new Bitfinex Stream.
func NewStream(ex *Exchange) *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
		depthBuffers:   make(map[string]*depth.Buffer),
		parser:         bfxapi.NewParser(),
		ex:             ex,
		logger:         log.WithField("module", "stream"),
	}
	stream.SetParser(stream.parser.Parse)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetEndpointCreator(stream.getEndpoint)
	stream.OnConnect(stream.onConnect)

	stream.OnTickerEvent(func(e *bfxapi.TickerEvent) {
		resp, ok := stream.parser.GetChannelResponse(e.ChannelID)
		if !ok {
			log.Errorf("unable to find channel response for channel ID: %d, ticker event: %+v", e.ChannelID, e)
			return
		}

		stream.EmitBookTickerUpdate(types.BookTicker{
			Symbol:   resp.Symbol,
			Buy:      e.Ticker.Bid,
			BuySize:  e.Ticker.BidSize,
			Sell:     e.Ticker.Ask,
			SellSize: e.Ticker.AskSize,
		})
	})

	stream.OnCandleEvent(func(e *bfxapi.CandleEvent) {
		// we need to get the response "key" to get the symbol and timeframe
		// e.g. "key": "trade:1m:tBTCUSD"
		resp, ok := stream.parser.GetChannelResponse(e.ChannelID)
		if !ok {
			log.Errorf("unable to find channel response for channel ID: %d, event %T: %+v", e.ChannelID, e, e)
			return
		} else if resp.Key == "" {
			log.Errorf("unable to find channel response key for channel ID: %d, event %T: %+v", e.ChannelID, e, e)
			return
		}

		// parse the key in format like "trade:1m:tBTCUSD"
		parts := bfxapi.ParseChannelKey(resp.Key)
		_, interval, symbol := parts[0], parts[1], parts[2]
		stream.EmitKLine(convertCandle(e.Candle, symbol, types.Interval(interval)))
	})

	stream.OnCandleSnapshotEvent(func(e *bfxapi.CandleSnapshotEvent) {})

	stream.OnStatusEvent(func(e *bfxapi.StatusEvent) {})

	stream.OnPublicTradeEvent(func(e *bfxapi.PublicTradeEvent) {
		resp, ok := stream.parser.GetChannelResponse(e.ChannelID)
		if !ok {
			log.Errorf("unable to find channel response for channel ID: %d, event %T: %+v", e.ChannelID, e, e)
			return
		} else if resp.Symbol == "" {
			log.Errorf("unable to find channel response symbol for channel ID: %d, event %T: %+v", e.ChannelID, e, e)
			return
		}

		stream.EmitMarketTrade(convertPublicTrade(e, resp))
	})

	stream.OnBookSnapshotEvent(func(e *bfxapi.BookSnapshotEvent) {
		resp, ok := stream.parser.GetChannelResponse(e.ChannelID)
		if !ok {
			log.Errorf("unable to find channel response for channel ID: %d, event %T: %+v", e.ChannelID, e, e)
			return
		} else if resp.Symbol == "" {
			log.Errorf("unable to find channel response key for channel ID: %d, event %T: %+v", e.ChannelID, e, e)
		}

		book := convertBookEntries(e.Entries, resp.Symbol)
		stream.EmitBookSnapshot(book)
	})

	stream.OnBookUpdateEvent(func(e *bfxapi.BookUpdateEvent) {
		resp, ok := stream.parser.GetChannelResponse(e.ChannelID)
		if !ok {
			log.Errorf("unable to find channel response for channel ID: %d, event %T: %+v", e.ChannelID, e, e)
			return
		} else if resp.Symbol == "" {
			log.Errorf("unable to find channel response key for channel ID: %d, event %T: %+v", e.ChannelID, e, e)
		}

		var book types.SliceOrderBook
		book.Symbol = toGlobalSymbol(resp.Symbol)

		if e.Entry.Amount.Sign() < 0 {
			book.Asks = types.PriceVolumeSlice{convertBookEntry(e.Entry)}
		} else {
			book.Bids = types.PriceVolumeSlice{convertBookEntry(e.Entry)}
		}

		stream.EmitBookUpdate(book)
	})

	stream.OnWalletSnapshotEvent(func(e *bfxapi.WalletSnapshotEvent) {
		stream.EmitBalanceUpdate(convertWallets(e.Wallets...))
	})
	stream.OnWalletUpdateEvent(func(e *bfxapi.Wallet) {
		stream.EmitBalanceUpdate(convertWallets(*e))
	})

	stream.OnOrderSnapshotEvent(func(e *bfxapi.UserOrderSnapshotEvent) {
		for _, uo := range e.Orders {
			order := convertWsUserOrder(&uo)
			if order != nil {
				stream.EmitOrderUpdate(*order)
			}
		}
	})

	stream.OnOrderUpdateEvent(func(e *bfxapi.UserOrder) {
		order := convertWsUserOrder(e)
		if order != nil {
			stream.EmitOrderUpdate(*order)
		}
	})

	stream.OnTradeUpdateEvent(func(e *bfxapi.TradeUpdateEvent) {
		trade := convertWsUserTrade(e)
		if trade != nil {
			stream.EmitTradeUpdate(*trade)
		}
	})

	return stream
}

// getEndpoint returns the websocket endpoint URL.
func (s *Stream) getEndpoint(ctx context.Context) (string, error) {
	url := os.Getenv("BITFINEX_API_WS_URL")
	if url == "" {
		if s.PublicOnly {
			url = bfxapi.PublicWebSocketURL
		} else {
			url = bfxapi.PrivateWebSocketURL
		}
	}
	return url, nil
}

// onConnect handles authentication for private websocket endpoint.
func (s *Stream) onConnect() {
	ctx := context.Background()
	endpoint, err := s.getEndpoint(ctx)
	if err != nil {
		s.logger.WithError(err).Error("bitfinex websocket: failed to get endpoint")
		return
	}

	if s.PublicOnly {
		if err := s.writeSubscriptions(); err != nil {
			log.WithError(err).Error("subscribe error")
		}
		return
	}

	if endpoint == bfxapi.PrivateWebSocketURL {
		apiKey := s.ex.apiKey
		apiSecret := s.ex.apiSecret
		if apiKey == "" || apiSecret == "" {
			s.logger.Warn("bitfinex private websocket: missing API key or secret")
		}

		authMsg := bfxapi.GenerateAuthRequest(apiKey, apiSecret)
		if err := s.Conn.WriteJSON(authMsg); err != nil {
			s.logger.WithError(err).Error("bitfinex auth: failed to send auth message")
			return
		}

		s.logger.Info("bitfinex private websocket: sent auth message")
	}
}

// writeSubscriptions send the subscription command to the websocket
// server in order to establish the connection to market data sources
func (s *Stream) writeSubscriptions() error {
	var reqs []*bfxapi.WebSocketRequest
	for _, subscription := range s.Subscriptions {
		req := convertSubscription(subscription)
		if req == nil {
			continue
		}

		reqs = append(reqs, req)
	}

	if len(reqs) == 0 {
		return nil
	}

	var err error
	for _, req := range reqs {
		s.logger.Infof("subscribing to channel: %+v", req)
		err = s.Conn.WriteJSON(req)
		if err != nil {
			s.logger.WithError(err).Errorf("failed to subscribe to channel: %+v", req)
		}
	}

	return err
}

// dispatchEvent dispatches parsed events to corresponding callbacks.
func (s *Stream) dispatchEvent(e interface{}) {
	switch evt := e.(type) {
	case nil:

	case *bfxapi.WebSocketResponse:
		s.EmitResponse(evt)

	case *bfxapi.HeartBeatEvent:
		s.EmitHeartBeat(evt)

	case *bfxapi.WalletSnapshotEvent:
		s.EmitWalletSnapshotEvent(evt)

	case *bfxapi.BalanceUpdateEvent:
		// balance update (balance update is actually AUM update in Bitfinex)
		s.EmitBalanceUpdateEvent(evt)

	case *bfxapi.UserPositionSnapshotEvent:
		s.EmitPositionSnapshotEvent(evt)

	case *bfxapi.UserPosition:
		s.EmitPositionUpdateEvent(evt)

	case *bfxapi.UserOrderSnapshotEvent:
		s.EmitOrderSnapshotEvent(evt)

	case *bfxapi.Wallet: // wallet update
		s.EmitWalletUpdateEvent(evt)

	case *bfxapi.UserOrder:
		s.EmitOrderUpdateEvent(evt)

	case *bfxapi.TradeUpdateEvent:
		s.EmitTradeUpdateEvent(evt)

	/* public data event */
	case *bfxapi.TickerEvent:
		s.EmitTickerEvent(evt)

	case *bfxapi.CandleSnapshotEvent:
		s.EmitCandleSnapshotEvent(evt)

	case *bfxapi.CandleEvent:
		s.EmitCandleEvent(evt)

	case *bfxapi.BookUpdateEvent:
		s.EmitBookUpdateEvent(evt)

	case *bfxapi.BookSnapshotEvent:
		s.EmitBookSnapshotEvent(evt)

	case *bfxapi.StatusEvent:
		s.EmitStatusEvent(evt)

	case *bfxapi.PublicTradeEvent:
		s.EmitPublicTradeEvent(evt)

	case *bfxapi.PublicTradeSnapshotEvent:
		s.EmitPublicTradeSnapshotEvent(evt)

	case *bfxapi.PublicFundingTradeEvent:
		s.EmitPublicFundingTradeEvent(evt)

	case *bfxapi.PublicFundingTradeSnapshotEvent:
		s.EmitPublicFundingTradeSnapshotEvent(evt)

	case *bfxapi.FundingBookSnapshotEvent:
		s.EmitFundingBookSnapshotEvent(evt)

	case *bfxapi.FundingBookUpdateEvent:
		s.EmitFundingBookEvent(evt)

	default:
		s.logger.Warnf("unhandled %T event: %+v", evt, evt)
	}
}

// convertWallets converts a Bitfinex Wallet to a types.Balance.
// It maps fields from Wallet to types.Balance.
func convertWallets(ws ...bfxapi.Wallet) types.BalanceMap {
	bm := types.BalanceMap{}
	for _, w := range ws {
		cu := toGlobalCurrency(w.Currency)
		bm[cu] = types.Balance{
			Currency:  cu,
			Available: w.AvailableBalance,
			Locked:    w.Balance.Sub(w.AvailableBalance),
			Interest:  w.UnsettledInterest,
		}
	}

	return bm
}

// convertWsUserOrder converts a Bitfinex websocket UserOrder to a types.Order.
// It maps fields from *bfxapi.UserOrder to types.Order.
func convertWsUserOrder(uo *bfxapi.UserOrder) *types.Order {
	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   uo.Symbol,
			Type:     toGlobalOrderType(uo.OrderType),
			Side:     toGlobalSide(uo.AmountOrig),
			Price:    uo.Price,
			Quantity: uo.AmountOrig.Abs(),
			ClientOrderID: func() string {
				if uo.CID != nil {
					return strconv.FormatInt(*uo.CID, 10)
				}

				return ""
			}(),
		},
		OrderID:          uint64(uo.OrderID),
		ExecutedQuantity: uo.AmountOrig.Abs().Sub(uo.Amount.Abs()),
		Status:           toGlobalOrderStatus(uo.Status),
		CreationTime:     types.Time(uo.CreatedAt.Time()),
		UpdateTime:       types.Time(uo.UpdatedAt.Time()),
	}
}

// convertWsUserTrade converts a Bitfinex websocket TradeUpdateEvent to a types.Trade.
// It maps fields from *bfxapi.TradeUpdateEvent to types.Trade.
func convertWsUserTrade(ut *bfxapi.TradeUpdateEvent) *types.Trade {
	return &types.Trade{
		ID:       uint64(ut.ID),
		OrderID:  uint64(ut.OrderID),
		Symbol:   ut.Symbol,
		Price:    ut.ExecPrice,
		Quantity: ut.ExecAmount.Abs(),
		Side:     toGlobalSide(ut.ExecAmount),
		Fee: func() fixedpoint.Value {
			if ut.Fee != nil {
				return *ut.Fee
			} else {
				return fixedpoint.Zero
			}
		}(),
		FeeCurrency: func() string {
			if ut.FeeCurrency != nil {
				return *ut.FeeCurrency
			} else {
				return ""
			}
		}(),
		IsMaker: ut.Maker == 1,
		Time:    types.Time(ut.Time.Time()),
	}
}

// convertPublicTrade converts a Bitfinex PublicTradeEvent to types.Trade.
// It uses the channel response to get the symbol.
func convertPublicTrade(e *bfxapi.PublicTradeEvent, resp *bfxapi.WebSocketResponse) types.Trade {
	var side types.SideType
	if e.Trade.Amount.Sign() >= 0 {
		side = types.SideTypeBuy
	} else {
		side = types.SideTypeSell
	}

	return types.Trade{
		ID:       uint64(e.Trade.ID),
		Exchange: ID,
		Symbol:   resp.Symbol,
		Price:    e.Trade.Price,
		Quantity: e.Trade.Amount.Abs(),
		Side:     side,
		IsMaker:  false, // public trade are taker, Bitfinex public trade does not provide maker info
		Time:     types.Time(e.Trade.Time),
	}
}

// convertSubscription converts a types.Subscription to bfxapi.WebSocketRequest.
// It maps channel, symbol, and options to the request fields.
// For types.KLineChannel, it builds req.Key as 'trade:TIMEFRAME:SYMBOL'.
func convertSubscription(sub types.Subscription) *bfxapi.WebSocketRequest {
	ch := convertChannel(sub.Channel)
	if ch == "" {
		return nil
	}

	req := &bfxapi.WebSocketRequest{
		Event:   "subscribe",
		Channel: ch,
		Symbol:  toLocalSymbol(sub.Symbol),
	}

	// For kline channel, build the key as 'trade:TIMEFRAME:SYMBOL'
	switch sub.Channel {
	case types.KLineChannel:
		interval := sub.Options.Interval
		if interval == "" {
			interval = "1m" // default to 1m if not specified
		}

		req.Key = "trade:" + interval.String() + ":" + toLocalSymbol(sub.Symbol)
	case types.BookChannel:
		req.Prec = "P0"
		req.Frequency = "F0"
		switch sub.Options.Depth {
		case types.DepthLevelMedium:
			req.Length = "100" // Number of price points ("1", "25", "100", "250") [default="25"]
		case types.DepthLevelFull:
			req.Length = "250" // Number of price points ("1", "25", "100", "250") [default="25"]
		default:
			req.Length = "25" // Number of price points ("1", "25", "100", "250") [default="25"]
		}
	case types.BookTickerChannel:
		req.Prec = "P0"
		req.Frequency = "F0"
		req.Length = "1" // Number of price points ("1", "25", "100", "250") [default="25"]
	case types.TickerChannel:
	}

	return req
}

// convertChannel converts types.Channel to bfxapi.Channel.
// It maps bbgo types.Channel constants to Bitfinex channel names.
func convertChannel(ch types.Channel) bfxapi.Channel {
	switch ch {
	case types.BookChannel:
		return bfxapi.ChannelBook

	case types.BookTickerChannel:
		return bfxapi.ChannelBook

	case types.TickerChannel:
		return bfxapi.ChannelTicker

	case types.MarketTradeChannel:
		return bfxapi.ChannelTrades

	case types.KLineChannel:
		return bfxapi.ChannelCandles

	default:
		return bfxapi.Channel(ch)
	}
}
