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

	tickerEventCallbacks []func(e *bfxapi.TickerEvent)

	bookUpdateEventCallbacks   []func(e *bfxapi.BookUpdateEvent)
	bookSnapshotEventCallbacks []func(e *bfxapi.BookSnapshotEvent)

	fundingBookEventCallbacks         []func(e *bfxapi.FundingBookUpdateEvent)
	fundingBookSnapshotEventCallbacks []func(e *bfxapi.FundingBookSnapshotEvent)

	candleEventCallbacks      []func(e *bfxapi.CandleEvent)
	statusEventCallbacks      []func(e *bfxapi.StatusEvent)
	marketTradeEventCallbacks []func(e *bfxapi.MarketTradeEvent)

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

// dispatchEvent dispatches parsed events to corresponding callbacks.
func (s *Stream) dispatchEvent(e interface{}) {
	switch evt := e.(type) {

	case *bfxapi.BookUpdateEvent: // book snapshot
		s.EmitBookUpdateEvent(evt)

	case *bfxapi.BookSnapshotEvent:
		s.EmitBookSnapshotEvent(evt)

	case *bfxapi.WalletSnapshotEvent:
		s.EmitBalanceUpdate(convertWallets(evt.Wallets...))

	case []bfxapi.UserOrder: // order snapshot
		for _, uo := range evt {
			order := convertWsUserOrder(&uo)
			if order != nil {
				s.EmitOrderUpdate(*order)
			}
		}

	case *bfxapi.Wallet: // wallet update
		s.EmitBalanceUpdate(convertWallets(*evt))

	case *bfxapi.UserOrder:
		order := convertWsUserOrder(evt)
		if order != nil {
			s.EmitOrderUpdate(*order)
		}

	case *bfxapi.UserTrade:
		trade := convertWsUserTrade(evt)
		if trade != nil {
			s.EmitTradeUpdate(*trade)
		}
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
			Price:    uo.Price,
			Quantity: uo.AmountOrig,
			ClientOrderID: func() string {
				if uo.CID != nil {
					return strconv.FormatInt(*uo.CID, 10)
				}

				return ""
			}(),
		},
		OrderID:          uint64(uo.OrderID),
		ExecutedQuantity: uo.AmountOrig.Sub(uo.Amount),
		Status:           types.OrderStatus(uo.Status),
		CreationTime:     types.Time(uo.CreatedAt.Time()),
		UpdateTime:       types.Time(uo.UpdatedAt.Time()),
	}
}

// convertWsUserTrade converts a Bitfinex websocket UserTrade to a types.Trade.
// It maps fields from *bfxapi.UserTrade to types.Trade.
func convertWsUserTrade(ut *bfxapi.UserTrade) *types.Trade {
	return &types.Trade{
		ID:       uint64(ut.ID),
		OrderID:  uint64(ut.OrderID),
		Symbol:   ut.Symbol,
		Price:    ut.ExecPrice,
		Quantity: ut.ExecAmount,
		Side: func() types.SideType {
			if ut.ExecAmount.Sign() > 0 {
				return types.SideTypeBuy
			} else {
				return types.SideTypeSell
			}
		}(),
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
