package mock

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Exchange struct {
	publicEx types.ExchangePublic

	userDataStream *types.StandardStream

	account *types.Account

	orderId uint64

	tradeId uint64

	openOrders map[uint64]types.Order

	markets types.MarketMap
}

func New(ex types.ExchangePublic, bals types.BalanceMap) *Exchange {
	stream := types.NewStandardStream()
	account := types.NewAccount()
	account.UpdateBalances(bals)

	return &Exchange{
		publicEx:       ex,
		userDataStream: &stream,
		account:        account,
		openOrders:     make(map[uint64]types.Order),
	}
}

func (e *Exchange) Initialize(ctx context.Context) error {
	markets, err := e.publicEx.QueryMarkets(ctx)
	if err != nil {
		return err
	}

	e.markets = markets
	return nil
}

func (e *Exchange) Name() types.ExchangeName {
	return e.publicEx.Name()
}

func (e *Exchange) PlatformFeeCurrency() string {
	return e.publicEx.PlatformFeeCurrency()
}

func (e *Exchange) NewStream() types.Stream {
	return e.publicEx.NewStream()
}

func (e *Exchange) NewUserDataStream() types.Stream {
	return e.userDataStream
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	return e.publicEx.QueryMarkets(ctx)
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	return e.publicEx.QueryTicker(ctx, symbol)
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	return e.publicEx.QueryTickers(ctx, symbol...)
}

func (e *Exchange) QueryKLines(
	ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions,
) ([]types.KLine, error) {
	return e.publicEx.QueryKLines(ctx, symbol, interval, options)
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	return e.account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	return e.account.Balances(), nil
}

// SubmitOrder simulates order submission
func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	market, ok := e.markets[order.Symbol]
	if !ok {
		return nil, fmt.Errorf("market not found: %s", order.Symbol)
	}

	var emptyMarket types.Market
	if order.Market == emptyMarket {
		order.Market = market
	}

	ticker, err := e.publicEx.QueryTicker(ctx, order.Symbol)
	if err != nil {
		return nil, err
	}

	// balance check and deduction/lock
	balances := e.account.Balances()
	switch order.Side {
	case types.SideTypeBuy:
		cost := order.Quantity.Mul(order.Price)
		quoteBal, ok := balances[market.QuoteCurrency]
		if !ok || quoteBal.Available.Compare(cost) < 0 {
			return nil, fmt.Errorf("insufficient quote balance: need %s %s, available %s", cost.String(), market.QuoteCurrency, quoteBal.Available.String())
		}
		switch order.Type {
		case types.OrderTypeMarket:
			quoteBal.Available = quoteBal.Available.Sub(cost)
		case types.OrderTypeLimit, types.OrderTypeLimitMaker, types.OrderTypeStopLimit, types.OrderTypeStopMarket:
			// 限價單鎖定
			quoteBal.Available = quoteBal.Available.Sub(cost)
			quoteBal.Locked = quoteBal.Locked.Add(cost)
		default:
			quoteBal.Available = quoteBal.Available.Sub(cost)
			quoteBal.Locked = quoteBal.Locked.Add(cost)
		}
		balances[market.QuoteCurrency] = quoteBal
	case types.SideTypeSell:
		baseBal, ok := balances[market.BaseCurrency]
		if !ok || baseBal.Available.Compare(order.Quantity) < 0 {
			return nil, fmt.Errorf("insufficient base balance: need %s %s, available %s", order.Quantity.String(), market.BaseCurrency, baseBal.Available.String())
		}
		switch order.Type {
		case types.OrderTypeMarket:
			baseBal.Available = baseBal.Available.Sub(order.Quantity)
		case types.OrderTypeLimit, types.OrderTypeLimitMaker, types.OrderTypeStopLimit, types.OrderTypeStopMarket:
			baseBal.Available = baseBal.Available.Sub(order.Quantity)
			baseBal.Locked = baseBal.Locked.Add(order.Quantity)
		default:
			baseBal.Available = baseBal.Available.Sub(order.Quantity)
			baseBal.Locked = baseBal.Locked.Add(order.Quantity)
		}
		balances[market.BaseCurrency] = baseBal
	}

	e.account.UpdateBalances(balances)
	e.userDataStream.EmitBalanceUpdate(balances)

	orderId := atomic.AddUint64(&e.orderId, 1)
	createdOrder = &types.Order{
		SubmitOrder: order,
		OrderID:     orderId,
		Status:      types.OrderStatusNew,
	}

	switch order.Type {
	case types.OrderTypeMarket:
		createdOrder.Status = types.OrderStatusFilled
		createdOrder.ExecutedQuantity = order.Quantity

		switch order.Side {
		case types.SideTypeBuy:
			createdOrder.AveragePrice = ticker.Sell
		case types.SideTypeSell:
			createdOrder.AveragePrice = ticker.Buy
		}

		trade := newMockTradeFromOrder(createdOrder, false)
		e.userDataStream.EmitTradeUpdate(trade)
		e.userDataStream.EmitOrderUpdate(*createdOrder)

		return createdOrder, nil
	}

	e.openOrders[e.orderId] = *createdOrder
	return createdOrder, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	return nil, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	return nil
}

// NewMockTradeFromOrder creates a mock trade from the given order
func newMockTradeFromOrder(order *types.Order, isMaker bool) types.Trade {
	price := order.Price

	if price.IsZero() && order.AveragePrice.Sign() > 0 {
		price = order.AveragePrice
	}

	return types.Trade{
		ID:            order.OrderID,
		OrderID:       order.OrderID,
		Exchange:      order.Exchange,
		Price:         price,
		Quantity:      order.Quantity,
		QuoteQuantity: order.Price.Mul(order.Quantity),
		Symbol:        order.Symbol,
		Side:          order.Side,
		IsBuyer:       order.Side == types.SideTypeBuy,
		IsMaker:       isMaker,
		Time:          types.Time(time.Now()),
		Fee:           fixedpoint.Zero,
		FeeCurrency:   order.Market.QuoteCurrency,
	}
}
